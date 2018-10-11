package cmd

import (
	"fmt"
	"os"

	"github.com/couchbase/cbauth/service"
	"github.com/couchbase/mobile-service"
	"github.com/spf13/cobra"
)

var cfgFile string

var (
	NodeUUID           string
	DataDir            string
	CouchbaseServerURL string
)

const (
	metaKvKeyPathRoot = "/mobile/"
	grpcListenPort    = 50051
	initCBGT          = false
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mobile-service",
	Short: "Runs the mobile-service ns-server service",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Printf("Mobile-service starting up.  NodeUUID: %s CouchbaseServerURL: %s.  DataDir: %v\n", NodeUUID, CouchbaseServerURL, DataDir)

		// Start GRPC server
		go mobile_mds.StartGrpcServer(service.NodeID(NodeUUID), grpcListenPort)

		// Start service manager
		hostport, err := mobile_mds.StripHttpScheme(CouchbaseServerURL)
		if err != nil {
			panic(fmt.Sprintf("ServiceManager error: %v", err))
		}
		hostportOffset, err := mobile_mds.AddPortOffset(hostport, 100)
		if err != nil {
			panic(fmt.Sprintf("ServiceManager error: %v", err))
		}
		mobile_mds.InitNode(service.NodeID(service.NodeID(NodeUUID)), hostportOffset)
		mobile_mds.RegisterManager() // Blocks forever

	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {

	rootCmd.PersistentFlags().StringVar(&DataDir, "dataDir", ".", "The relative directory where data should be stored")
	rootCmd.PersistentFlags().StringVar(&NodeUUID, "uuid", "", "The node uuid of this Couchbase CouchbaseServerURL node")
	rootCmd.PersistentFlags().StringVar(&CouchbaseServerURL, "server", "http://localhost:8091", "The Couchbase CouchbaseServerURL url to connect to")

}
