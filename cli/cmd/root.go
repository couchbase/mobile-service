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
	GrpcTlsPort        int
	RestAdminPort      int
	RestAdminPortTLS   int
	EnterpriseEdition  bool
)

const (
	metaKvKeyPathRoot = "/mobile/"
	initCBGT          = false
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mobile-service",
	Short: "Runs the mobile-service ns-server service.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Printf("Mobile-service starting up.  NodeUUID: %s CouchbaseServerURL: %s.  DataDir: %v.  EE: %v\n", NodeUUID, CouchbaseServerURL, DataDir, EnterpriseEdition)

		// Start GRPC server
		go mobile_service.StartGrpcServer(service.NodeID(NodeUUID), GrpcTlsPort)

		// Register manager to receive ns-server lifecycle callbacks
		mobile_service.InitNode(service.NodeID(service.NodeID(NodeUUID)), CouchbaseServerURL)
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

	rootCmd.PersistentFlags().StringVar(
		&DataDir,
		"dataDir",
		".",
		"The relative directory where data should be stored",
	)

	rootCmd.PersistentFlags().StringVar(
		&NodeUUID,
		"uuid",
		"",
		"The node uuid of this Couchbase CouchbaseServerURL node",
	)

	rootCmd.PersistentFlags().StringVar(
		&CouchbaseServerURL,
		"server",
		"http://localhost:8091",
		"The Couchbase CouchbaseServerURL url to connect to",
	)

	rootCmd.PersistentFlags().IntVar(
		&GrpcTlsPort,
		"grpcTlsPort",
		18098,
		"The TLS encrypted grpc server port",
	)

	rootCmd.PersistentFlags().IntVar(
		&RestAdminPort,
		"restAdminPort",
		8097,
		"The REST Admin port to support UI functionality and tooling",
	)

	rootCmd.PersistentFlags().IntVar(
		&RestAdminPortTLS,
		"restAdminPortTLS",
		18097,
		"The TLS encrypted REST Admin port to support UI functionality and tooling",
	)

	rootCmd.PersistentFlags().BoolVar(
		&EnterpriseEdition,
		"enterprise",
		false,
		"Start mobile-service in Enterprise Edition (EE) mode",
	)

}
