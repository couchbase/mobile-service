package mobile_mds

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sort"

	"hash/crc32"

	"bytes"
	"container/list"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/couchbase/cbauth/metakv"
	"github.com/couchbase/cbauth/service"
	"encoding/binary"
)

// This is a copy of the example caching service which is being used as a starting point for the mobile
// service for the purposes of interfacing with ns-server's service API.
//
// The "tokens" are superfluous and residue from the example caching service, and should be ripped out.



var (
	MyNode service.NodeID
	MyHost string
)

const (
	ServiceDir    = "/mobile-service/"
	TokensPerNode = 256
	TokensKey     = ServiceDir + "tokens"
)

func InitNode(node service.NodeID, host string) {
	MyNode = node
	MyHost = host

	SetNodeHostName(node, host)
	MaybeCreateInitialTokenMap()
}

type Mgr struct {
	mu      *sync.RWMutex
	waiters waiters

	tokenMapStream *TokenMapStream
	cache          *Cache

	rebalancer   *Rebalancer
	rebalanceCtx *rebalanceContext

	state
}

type rebalanceContext struct {
	change service.TopologyChange
	rev    uint64
}

func (ctx *rebalanceContext) incRev() uint64 {
	curr := ctx.rev
	ctx.rev++

	return curr
}

type waiter chan state
type waiters map[waiter]struct{}

type state struct {
	rev    uint64
	tokens *TokenMap

	rebalanceID   string
	rebalanceTask *service.Task
}

func NewMgr() *Mgr {
	mu := &sync.RWMutex{}

	tokenMapStream := NewTokenMapStream()
	tokens := <-tokenMapStream.C

	cache := NewCache()

	mgr := &Mgr{
		mu:             mu,
		waiters:        make(waiters),
		tokenMapStream: tokenMapStream,
		state: state{
			rev:           0,
			tokens:        tokens,
			rebalanceID:   "",
			rebalanceTask: nil,
		},
	}

	go mgr.tokenMapStreamReader()

	httpAPI := HTTPAPI{mgr, cache}
	go httpAPI.ListenAndServe()

	return mgr
}

func RegisterManager() {
	mgr := NewMgr()
	err := service.RegisterManager(mgr, nil)
	if err != nil {
		log.Fatalf("Couldn't register service manager: %s", err.Error())
	}
}

func (m *Mgr) GetCurrentTokenMap() *TokenMap {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getCurrentTokenMapLOCKED()
}

func (m *Mgr) getCurrentTokenMapLOCKED() *TokenMap {
	return m.copyStateLOCKED().tokens
}

func (m *Mgr) GetNodeInfo() (*service.NodeInfo, error) {
	opaque := struct {
		Host string `json:"host"`
	}{
		MyHost,
	}

	info := &service.NodeInfo{
		NodeID:   MyNode,
		Priority: 0,
		Opaque:   opaque,
	}

	log.Printf("GetNodeInfo() returning: %+v", info)

	return info, nil
}

func (m *Mgr) Shutdown() error {

	log.Printf("Shutdown() called, calling os.Exit()")

	os.Exit(0)

	return nil
}

func (m *Mgr) GetTaskList(rev service.Revision,
	cancel service.Cancel) (*service.TaskList, error) {

	log.Printf("GetTaskList called, rev: %v cancel: %v", rev, cancel)

	state, err := m.wait(rev, cancel)
	if err != nil {
		return nil, err
	}

	taskList := stateToTaskList(state)

	log.Printf("GetTaskList returning: %+v", taskList)

	return taskList, nil
}

func (m *Mgr) CancelTask(id string, rev service.Revision) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("CancelTask called.  id: %v, rev: %v", id, rev)

	tasks := stateToTaskList(m.state).Tasks
	task := (*service.Task)(nil)

	for i := range tasks {
		t := &tasks[i]

		if t.ID == id {
			task = t
			break
		}
	}

	if task == nil {
		return service.ErrNotFound
	}

	if !task.IsCancelable {
		return service.ErrNotSupported
	}

	if rev != nil && !bytes.Equal(rev, task.Rev) {
		return service.ErrConflict
	}

	return m.cancelActualTaskLOCKED(task)
}

func (m *Mgr) GetCurrentTopology(rev service.Revision,
	cancel service.Cancel) (*service.Topology, error) {

	log.Printf("ServiceManager.GetCurrentTopology() called rev: %+v", rev)

	state, err := m.wait(rev, cancel)
	if err != nil {
		return nil, err
	}

	topology := stateToTopology(state)
	fmt.Printf("GetCurrentTopology() returning: %+v", topology)
	return topology, nil
}

func (m *Mgr) PrepareTopologyChange(change service.TopologyChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("PrepareTopologyChange() called with: %+v", change)

	if m.state.rebalanceID != "" {
		return service.ErrConflict
	}

	m.updateStateLOCKED(func(s *state) {
		s.rebalanceID = change.ID
	})

	return nil
}

func (m *Mgr) StartTopologyChange(change service.TopologyChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("StartTopologyChange() called with: %+v", change)

	if m.state.rebalanceID != change.ID || m.rebalancer != nil {
		return service.ErrConflict
	}

	if change.CurrentTopologyRev != nil {
		haveRev := DecodeRev(change.CurrentTopologyRev)
		if haveRev != m.state.rev {
			return service.ErrConflict
		}
	}

	ctx := &rebalanceContext{
		rev:    0,
		change: change,
	}

	m.rebalanceCtx = ctx
	m.updateRebalanceProgressLOCKED(0)

	tokens := m.getCurrentTokenMapLOCKED()
	rebalancer := NewRebalancer(tokens, change,
		m.rebalanceProgressCallback, m.rebalanceDoneCallback)

	m.rebalancer = rebalancer

	return nil
}

func (m *Mgr) runRebalanceCallback(cancel <-chan struct{}, body func()) {
	done := make(chan struct{})

	go func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		select {
		case <-cancel:
			break
		default:
			body()
		}

		close(done)
	}()

	select {
	case <-done:
	case <-cancel:
	}
}

func (m *Mgr) rebalanceProgressCallback(progress float64, cancel <-chan struct{}) {
	m.runRebalanceCallback(cancel, func() {
		m.updateRebalanceProgressLOCKED(progress)
	})
}

func (m *Mgr) updateRebalanceProgressLOCKED(progress float64) {
	rev := m.rebalanceCtx.incRev()
	changeID := m.rebalanceCtx.change.ID
	task := &service.Task{
		Rev:          EncodeRev(rev),
		ID:           fmt.Sprintf("rebalance/%s", changeID),
		Type:         service.TaskTypeRebalance,
		Status:       service.TaskStatusRunning,
		IsCancelable: true,
		Progress:     progress,

		Extra: map[string]interface{}{
			"rebalanceId": changeID,
		},
	}

	m.updateStateLOCKED(func(s *state) {
		s.rebalanceTask = task
	})
}

func (m *Mgr) rebalanceDoneCallback(err error, cancel <-chan struct{}) {
	m.runRebalanceCallback(cancel, func() { m.onRebalanceDoneLOCKED(err) })
}

func (m *Mgr) onRebalanceDoneLOCKED(err error) {
	newTask := (*service.Task)(nil)
	if err != nil {
		ctx := m.rebalanceCtx
		rev := ctx.incRev()

		newTask = &service.Task{
			Rev:          EncodeRev(rev),
			ID:           fmt.Sprintf("rebalance/%s", ctx.change.ID),
			Type:         service.TaskTypeRebalance,
			Status:       service.TaskStatusFailed,
			IsCancelable: true,

			ErrorMessage: err.Error(),

			Extra: map[string]interface{}{
				"rebalanceId": ctx.change.ID,
			},
		}
	}

	m.rebalancer = nil
	m.rebalanceCtx = nil

	m.updateStateLOCKED(func(s *state) {
		s.rebalanceTask = newTask
		s.rebalanceID = ""
	})
}

func (m *Mgr) notifyWaitersLOCKED() {
	s := m.copyStateLOCKED()
	for ch := range m.waiters {
		if ch != nil {
			ch <- s
		}
	}

	m.waiters = make(waiters)
}

func (m *Mgr) addWaiterLOCKED() waiter {
	ch := make(waiter, 1)
	m.waiters[ch] = struct{}{}

	return ch
}

func (m *Mgr) removeWaiter(w waiter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.waiters, w)
}

func (m *Mgr) tokenMapStreamReader() {
	for {
		select {
		case tokens := <-m.tokenMapStream.C:
			if m.tokenMapStream.IsCanceled() {
				return
			}

			m.updateState(func(s *state) {
				s.tokens = tokens
			})
		}
	}
}

func (m *Mgr) updateState(body func(state *state)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateStateLOCKED(body)
}

func (m *Mgr) updateStateLOCKED(body func(state *state)) {
	body(&m.state)
	m.state.rev++

	m.notifyWaitersLOCKED()
}

func (m *Mgr) wait(rev service.Revision,
	cancel service.Cancel) (state, error) {

	m.mu.Lock()

	unlock := NewCleanup(func() { m.mu.Unlock() })
	defer unlock.Run()

	currState := m.copyStateLOCKED()

	if rev == nil {
		return currState, nil
	}

	haveRev := DecodeRev(rev)
	if haveRev != m.rev {
		return currState, nil
	}

	ch := m.addWaiterLOCKED()
	unlock.Run()

	select {
	case <-cancel:
		m.removeWaiter(ch)
		return state{}, service.ErrCanceled
	case newState := <-ch:
		return newState, nil
	}
}

func (m *Mgr) copyStateLOCKED() state {
	s := m.state
	s.tokens = s.tokens.Copy()

	return s
}

func (m *Mgr) cancelActualTaskLOCKED(task *service.Task) error {
	switch task.Type {
	case service.TaskTypePrepared:
		return m.cancelPrepareTaskLOCKED()
	case service.TaskTypeRebalance:
		return m.cancelRebalanceTaskLOCKED(task)
	default:
		panic("can't happen")
	}
}

func (m *Mgr) cancelPrepareTaskLOCKED() error {
	if m.rebalancer != nil {
		return service.ErrConflict
	}

	m.updateStateLOCKED(func(s *state) {
		s.rebalanceID = ""
	})

	return nil
}

func (m *Mgr) cancelRebalanceTaskLOCKED(task *service.Task) error {
	switch task.Status {
	case service.TaskStatusRunning:
		return m.cancelRunningRebalanceTaskLOCKED()
	case service.TaskStatusFailed:
		return m.cancelFailedRebalanceTaskLOCKED()
	default:
		panic("can't happen")
	}
}

func (m *Mgr) cancelRunningRebalanceTaskLOCKED() error {
	m.rebalancer.Cancel()
	m.onRebalanceDoneLOCKED(nil)

	return nil
}

func (m *Mgr) cancelFailedRebalanceTaskLOCKED() error {
	m.updateStateLOCKED(func(s *state) {
		s.rebalanceTask = nil
	})

	return nil
}

func stateToTopology(s state) *service.Topology {
	topology := &service.Topology{}

	servers := s.tokens.Servers

	topology.Rev = EncodeRev(s.rev)
	topology.Nodes = servers
	topology.IsBalanced = true

	if len(servers) <= 1 {
		topology.Messages = []string{
			"Not enough nodes to achieve awesomeness",
		}
	} else {
		topology.Messages = nil
	}

	return topology
}

func stateToTaskList(s state) *service.TaskList {
	tasks := &service.TaskList{}

	tasks.Rev = EncodeRev(s.rev)
	tasks.Tasks = make([]service.Task, 0)

	if s.rebalanceID != "" {
		id := s.rebalanceID

		task := service.Task{
			Rev:          EncodeRev(0),
			ID:           fmt.Sprintf("prepare/%s", id),
			Type:         service.TaskTypePrepared,
			Status:       service.TaskStatusRunning,
			IsCancelable: true,

			Extra: map[string]interface{}{
				"rebalanceId": id,
			},
		}

		tasks.Tasks = append(tasks.Tasks, task)
	}

	if s.rebalanceTask != nil {
		tasks.Tasks = append(tasks.Tasks, *s.rebalanceTask)
	}

	return tasks
}

func MetakvGet(path string, v interface{}) (bool, error) {
	raw, _, err := metakv.Get(path)
	if err != nil {
		return false, err
		// log.Fatalf("Failed to fetch %s from metakv: %s", path, err.Error())
	}

	if raw == nil {
		return false, nil
	}

	err = json.Unmarshal(raw, v)
	if err != nil {
		//log.Fatalf("Failed unmarshalling value for %s: %s\n%s",
		//	path, err.Error(), string(raw))
		return false, err
	}

	return true, nil
}

func MetakvSet(path string, v interface{}) (err error) {
	raw, err := json.Marshal(v)
	if err != nil {
		//log.Fatalf("Failed to marshal value for %s: %s\n%v",
		//	path, err.Error(), v)
		return err
	}

	err = metakv.Set(path, raw, nil)
	if err != nil {
		// log.Fatalf("Failed to set %s: %s", path, err.Error())
		return err
	}

	return nil
}

type Token struct {
	Point  uint32
	Server service.NodeID
}

type TokenList []Token

func (tl TokenList) Len() int {
	return len(tl)
}

func (tl TokenList) Less(i, j int) bool {
	return tl[i].Point < tl[j].Point
}

func (tl TokenList) Swap(i, j int) {
	tl[i], tl[j] = tl[j], tl[i]
}

type TokenMap struct {
	Servers []service.NodeID
	Tokens  TokenList
}

func (tm TokenMap) save() {
	MetakvSet(TokensKey, tm)
}

func MaybeCreateInitialTokenMap() {
	tm := &TokenMap{}
	foundTokenMap, _ := MetakvGet(TokensKey, tm)
	if foundTokenMap {
		return
	}

	log.Printf("No token map found. Creating initial one.")

	tm.Servers = []service.NodeID{}
	tm.Tokens = TokenList{}

	tm.save()
}

func (tm TokenMap) UpdateServers(newServers []service.NodeID) {

	log.Printf("TokenMap UpdateServers() called with: %v", newServers)

	removed := serversMap(tm.Servers)
	added := serversMap(newServers)

	for _, newServer := range newServers {
		delete(removed, newServer)
	}

	for _, oldServer := range tm.Servers {
		delete(added, oldServer)
	}

	newTokens := TokenList(nil)
	for _, token := range tm.Tokens {
		if _, ok := removed[token.Server]; ok {
			continue
		}

		newTokens = append(newTokens, token)
	}

	for addedServer := range added {
		newTokens = append(newTokens, createTokens(addedServer)...)
	}

	tm.Servers = append([]service.NodeID{}, newServers...)
	tm.Tokens = newTokens

	sort.Sort(tm.Tokens)
	tm.save()
}

func (tm TokenMap) FindOwner(key string) service.NodeID {
	h := hash(key)

	numTokens := len(tm.Tokens)
	i := sort.Search(numTokens,
		func(i int) bool {
			return tm.Tokens[i].Point > h
		})

	if i >= numTokens {
		i = 0
	}

	return tm.Tokens[i].Server
}

func (tm TokenMap) Copy() *TokenMap {
	cp := &TokenMap{}

	cp.Servers = append([]service.NodeID{}, tm.Servers...)
	cp.Tokens = append(TokenList{}, tm.Tokens...)

	return cp
}

func hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

func createTokens(node service.NodeID) TokenList {
	tokens := TokenList(nil)

	for i := 0; i < TokensPerNode; i++ {
		tokens = append(tokens, Token{rand.Uint32(), node})
	}

	return tokens
}

func serversMap(servers []service.NodeID) map[service.NodeID]struct{} {
	m := make(map[service.NodeID]struct{})

	for _, server := range servers {
		m[server] = struct{}{}
	}

	return m
}

type TokenMapStream struct {
	C      <-chan *TokenMap
	cancel chan struct{}
}

func NewTokenMapStream() *TokenMapStream {
	cancel := make(chan struct{})
	ch := make(chan *TokenMap)

	cb := func(path string, value []byte, rev interface{}) error {
		if path != TokensKey {
			return nil
		}

		tokens := &TokenMap{}
		err := json.Unmarshal(value, tokens)
		if err != nil {
			log.Fatalf("Failed to unmarshal token map: %s\n%s",
				err.Error(), string(value))
		}

		select {
		case ch <- tokens:
		case <-cancel:
		}

		return nil
	}

	go metakv.RunObserveChildren(ServiceDir, cb, cancel)

	return &TokenMapStream{
		C:      ch,
		cancel: cancel,
	}
}

func (s *TokenMapStream) IsCanceled() bool {
	select {
	case <-s.cancel:
		return true
	default:
		return false
	}
}

func (s *TokenMapStream) Cancel() {
	close(s.cancel)
}

const (
	MaxCacheSize = 1024
)

var (
	ErrKeyNotFound = errors.New("Key not found")
)

type item struct {
	key     string
	value   string
	lruElem *list.Element
}

type Cache struct {
	mu    *sync.Mutex
	lru   *list.List
	items map[string]*item

	maxSize int
}

func NewCache() *Cache {
	return &Cache{
		mu:      &sync.Mutex{},
		lru:     list.New(),
		items:   make(map[string]*item),
		maxSize: MaxCacheSize,
	}
}

func (c *Cache) Get(key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	itm, ok := c.items[key]
	if !ok {
		return "", ErrKeyNotFound
	}

	return itm.value, nil
}

func (c *Cache) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	itm, ok := c.items[key]
	if !ok {
		c.create(key, value)
		return
	}

	itm.value = value
	c.touch(itm)
}

func (c *Cache) maybeEvict() {
	if len(c.items) < c.maxSize {
		return
	}

	victim := c.lru.Remove(c.lru.Front()).(*item)
	delete(c.items, victim.key)
}

func (c *Cache) create(key string, value string) {
	c.maybeEvict()

	itm := &item{
		key:   key,
		value: value,
	}

	itm.lruElem = c.lru.PushBack(itm)

	c.items[key] = itm
}

func (c *Cache) touch(itm *item) {
	c.lru.MoveToBack(itm.lruElem)
}

type DoneCallback func(err error, cancel <-chan struct{})
type ProgressCallback func(progress float64, cancel <-chan struct{})

type Callbacks struct {
	progress ProgressCallback
	done     DoneCallback
}

type Rebalancer struct {
	tokens *TokenMap
	change service.TopologyChange

	cb Callbacks

	cancel chan struct{}
	done   chan struct{}
}

func NewRebalancer(tokens *TokenMap, change service.TopologyChange,
	progress ProgressCallback, done DoneCallback) *Rebalancer {

	r := &Rebalancer{
		tokens: tokens,
		change: change,
		cb:     Callbacks{progress, done},

		cancel: make(chan struct{}),
		done:   make(chan struct{}),
	}

	go r.doRebalance()
	return r
}

func (r *Rebalancer) Cancel() {
	close(r.cancel)
	<-r.done
}

func (r *Rebalancer) doRebalance() {

	log.Printf("doRebalance() called")

	defer close(r.done)

	isInitial := (len(r.tokens.Servers) == 0)
	isFailover := (r.change.Type == service.TopologyChangeTypeFailover)

	if !(isFailover || isInitial) {
		// make failover and initial rebalance fast
		r.fakeProgress()
	}

	r.removeDeadNodesMetaKV()

	r.updateHostNames()
	r.updateTokenMap()
	r.cb.done(nil, r.cancel)
}

func (r *Rebalancer) fakeProgress() {
	seconds := 20
	progress := float64(0)
	increment := 1.0 / float64(seconds)

	r.cb.progress(progress, r.cancel)

	for i := 0; i < seconds; i++ {
		select {
		case <-time.After(1 * time.Second):
			progress += increment
			r.cb.progress(progress, r.cancel)
		case <-r.cancel:
			return
		}
	}
}

func (r *Rebalancer) removeDeadNodesMetaKV() {

	log.Printf("Rebalancer removeDeadNodesMetaKV() called")

	// For each entry in ejected nodes, delete /mobile/state/<node-uuid>
	for _, ejectedNode := range r.change.EjectNodes {
		keypath := fmt.Sprintf("/mobile/state/%s/", ejectedNode.NodeID)
		log.Printf("Metakv delete recursive: %v", keypath)
		if err := metakv.RecursiveDelete(keypath); err != nil {
			log.Printf("Error deleting metkv key: %+v.  Err: %v", keypath, err)
		}
	}



}

func (r *Rebalancer) updateHostNames() {
	for _, node := range r.change.KeepNodes {
		id := node.NodeInfo.NodeID
		opaque := node.NodeInfo.Opaque.(map[string]interface{})
		host := opaque["host"].(string)

		SetNodeHostName(id, host)
	}
}


func (r *Rebalancer) updateTokenMap() {

	log.Printf("updateTokenMap()")

	nodes := []service.NodeID(nil)

	for _, node := range r.change.KeepNodes {
		log.Printf("updateTokenMap() adding node: %+v", node)

		nodes = append(nodes, node.NodeInfo.NodeID)
	}

	r.tokens.UpdateServers(nodes)
}


type HTTPAPI struct {
	mgr   *Mgr
	cache *Cache
}

func (h *HTTPAPI) DispatchCache(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET", "POST":
		break
	default:
		methodNotAllowed(rw).Write()
		return
	}

	key := strings.TrimPrefix(req.URL.Path, "/cache/")
	if key == "" {
		notFound(rw).Write()
		return
	}

	owner := h.mgr.GetCurrentTokenMap().FindOwner(key)
	if owner != MyNode {
		host := GetNodeHostName(owner)
		redirect(rw, req, host).Write()
		return
	}

	switch req.Method {
	case "GET":
		h.get(rw, key)
	case "POST":
		h.set(rw, req, key)
	default:
		panic("can't happen")
	}
}

func (h *HTTPAPI) get(rw http.ResponseWriter, key string) {
	v, err := h.cache.Get(key)
	if err != nil {
		if err == ErrKeyNotFound {
			notFound(rw).Write()
		} else {
			internalError(rw).Body(err.Error()).Write()
		}

		return
	}

	ok(rw).Body(v).Write()
}

func (h *HTTPAPI) set(rw http.ResponseWriter, req *http.Request, key string) {
	value, err := ioutil.ReadAll(req.Body)
	if err != nil {
		internalError(rw).Body("Internal server error: " + err.Error()).Write()
		return
	}

	h.cache.Set(key, string(value))
	ok(rw).Write()
}

type ConcreteToken struct {
	Point uint32 `json:"point"`
	Host  string `json:"host"`
}

func (h *HTTPAPI) TokenMap(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		methodNotAllowed(rw).Write()
		return
	}

	tokens := h.mgr.GetCurrentTokenMap().Tokens
	resp := []ConcreteToken(nil)

	hostnames := make(map[service.NodeID]string)
	for _, token := range tokens {
		server := token.Server
		host, ok := hostnames[server]
		if !ok {
			host = GetNodeHostName(server)
			if host == "" {
				msg := fmt.Sprintf("Internal server error: "+
					"no hostname for %s", server)
				internalError(rw).Body(msg).Write()
				return
			}

			hostnames[server] = host
		}

		resp = append(resp, ConcreteToken{token.Point, host})
	}

	ok(rw).JSON(resp).Write()
}

func ok(rw http.ResponseWriter) *Response {
	return NewResponse(rw).Status(http.StatusOK)
}

func notFound(rw http.ResponseWriter) *Response {
	return NewResponse(rw).Status(http.StatusNotFound).Body("Not found")
}

func internalError(rw http.ResponseWriter) *Response {
	return NewResponse(rw).Status(http.StatusInternalServerError)
}

func methodNotAllowed(rw http.ResponseWriter) *Response {
	return NewResponse(rw).Body("Method not allowed").Status(http.StatusMethodNotAllowed)
}

func extractKey(path string) string {
	return strings.TrimPrefix(path, "/cache/")
}

func redirect(rw http.ResponseWriter, req *http.Request, host string) *Response {
	url := *req.URL
	url.Scheme = "http"
	url.Host = host

	resp := NewResponse(rw)
	resp.Header("Location", url.String())
	resp.Header("Cache-Control", "must-revalidate")

	return resp.Status(http.StatusFound)
}

func (h *HTTPAPI) ListenAndServe() {
	http.HandleFunc("/cache/", h.DispatchCache)
	http.HandleFunc("/tokenMap", h.TokenMap)

	err := http.ListenAndServe(MyHost, nil)
	if err != nil {
		log.Fatal(err)
	}
}


type Response struct {
	writer http.ResponseWriter

	status  int
	body    []byte
	headers map[string]string
}

func NewResponse(rw http.ResponseWriter) *Response {
	return &Response{
		writer:  rw,
		status:  http.StatusOK,
		body:    nil,
		headers: make(map[string]string),
	}
}

func (resp *Response) Status(status int) *Response {
	resp.status = status
	return resp
}

func (resp *Response) Header(name, value string) *Response {
	resp.headers[name] = value
	return resp
}

func (resp *Response) Body(value string) *Response {
	resp.body = []byte(value)
	return resp
}

func (resp *Response) JSON(v interface{}) *Response {
	resp.Header("Content-Type", "application/json")
	json, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("Failed to marshal: %s\n%v", err.Error(), v)
	}

	resp.body = json

	return resp
}

func (resp *Response) Write() error {
	headers := resp.writer.Header()
	for name, value := range resp.headers {
		headers.Set(name, value)
	}

	resp.writer.WriteHeader(resp.status)
	_, err := resp.writer.Write([]byte(resp.body))

	return err
}



type Cleanup struct {
	canceled bool
	f        func()
}

func NewCleanup(f func()) *Cleanup {
	return &Cleanup{
		canceled: false,
		f:        f,
	}
}

func (c *Cleanup) Run() {
	if !c.canceled {
		c.f()
		c.Cancel()
	}
}

func (c *Cleanup) Cancel() {
	c.canceled = true
}



func SetNodeHostName(node service.NodeID, host string) {
	currentHost := GetNodeHostName(node)
	if host != currentHost {
		err := MetakvSet(hostPath(node), host)
		fmt.Printf("Error setting node hostname: %v", err)
	}
}

func GetNodeHostName(node service.NodeID) string {
	host := ""
	_, err := MetakvGet(hostPath(node), &host)
	if err != nil {
		return ""
	}

	return host
}

func hostPath(node service.NodeID) string {
	return fmt.Sprintf("%snodes/%s/host", ServiceDir, node)
}

func EncodeRev(rev uint64) service.Revision {
	ext := make(service.Revision, 8)
	binary.BigEndian.PutUint64(ext, rev)

	return ext
}

func DecodeRev(ext service.Revision) uint64 {
	return binary.BigEndian.Uint64(ext)
}
