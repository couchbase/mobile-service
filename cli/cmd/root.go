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
	initCBGT          = false
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mobile-service",
	Short: "Runs the mobile-service ns-server service",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Printf("Mobile-service starting up.  NodeUUID: %s CouchbaseServerURL: %s.  DataDir: %v\n", NodeUUID, CouchbaseServerURL, DataDir)

		// Start service manager
		hostport, err := mobile_service.StripHttpScheme(CouchbaseServerURL)
		if err != nil {
			panic(fmt.Sprintf("ServiceManager error: %v", err))
		}

		grpcPort, err := mobile_service.CalculateGrpcPort(hostport)
		fmt.Printf("Calculated grpc port to be: %v", grpcPort)
		if err != nil {
			panic(fmt.Sprintf("ServiceManager error: %v", err))
		}

		// Start GRPC server
		go mobile_service.StartGrpcServer(service.NodeID(NodeUUID), grpcPort)

		hostportOffset, err := mobile_service.AddPortOffset(hostport, 100)
		if err != nil {
			panic(fmt.Sprintf("ServiceManager error: %v", err))
		}
		mobile_service.InitNode(service.NodeID(service.NodeID(NodeUUID)), hostportOffset)
		mobile_service.RegisterManager() // Blocks forever

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
