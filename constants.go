package mobile_service

const (

	// Not used yet -- will be UI rest api ports
	PortRest    = 8097
	PortRestTls = 18097

	// We want the grpc port to be 18098 by default.
	// Use an offset approach where we calculate the grpc port based on an offset from the
	// default port of 8091.  If cbs runs on port 9000, as in the cluster_run example, then
	// the grpc port will be 9000 + 10007 = 19007
	PortGrpcTlsOffset = 10007
)

const (
	KeyMobileRoot             = "/mobile"
	KeyMobileGateway          = "/mobile/gateway"
	KeyMobileGatewayListener  = "/mobile/gateway/config/listener"
	KeyMobileGatewayGeneral   = "/mobile/gateway/config/general"
	KeyMobileGatewayDatabases = "/mobile/gateway/config/databases"
)

var (
	KeyDirMobileRoot             = AddTrailingSlash(KeyMobileRoot)
	KeyDirMobileGateway          = AddTrailingSlash(KeyMobileGateway)
	KeyDirMobileGatewayListener  = AddTrailingSlash(KeyMobileGatewayListener)
	KeyDirMobileGatewayGeneral   = AddTrailingSlash(KeyMobileGatewayGeneral)
	KeyDirMobileGatewayDatabases = AddTrailingSlash(KeyMobileGatewayDatabases)
)
