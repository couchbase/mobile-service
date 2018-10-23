package mobile_mds

const (

	PortRest = 8097
	PortRestTls = 18097
	PortGrpcTls = 18098

)


const (
	KeyMobileRoot = "/mobile"
	KeyMobileGateway = "/mobile/gateway"
	KeyMobileGatewayListener = "/mobile/gateway/listener"
	KeyMobileGatewayGeneral = "/mobile/gateway/general"
	KeyMobileGatewayDatabases = "/mobile/gateway/databases"
)

var (

	KeyDirMobileRoot = AddTrailingSlash(KeyMobileRoot)
	KeyDirMobileGateway = AddTrailingSlash(KeyMobileGateway)
	KeyDirMobileGatewayListener = AddTrailingSlash(KeyMobileGatewayListener)
	KeyDirMobileGatewayGeneral = AddTrailingSlash(KeyMobileGatewayGeneral)
	KeyDirMobileGatewayDatabases = AddTrailingSlash(KeyMobileGatewayDatabases)

)