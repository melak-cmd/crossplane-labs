package function

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/function-sdk-go"
	"github.com/melak-cmd/crossplane-labs/functions/function-recovery/internal/kubernetes"
)

type CLI struct {
	Debug                bool   `help:"Emit debug logs in addition to info logs." short:"d"`
	Network              string `default:"tcp" help:"Network on which to listen for gRPC connections."`
	Address              string `default:":9443" help:"Address at which to listen for gRPC connections."`
	TLSCertsDir          string `env:"TLS_SERVER_CERTS_DIR" help:"Directory containing server certs."`
	Insecure             bool   `help:"Run without mTLS credentials."`
	MaxRecvMessageSizeMB int    `default:"4" help:"Maximum received message size in MB."`
}

func (c *CLI) Run() error {
	log, err := function.NewLogger(c.Debug)
	if err != nil {
		return err
	}
	return function.Serve(
		New(log, kubernetes.New()),
		function.Listen(c.Network, c.Address),
		function.MTLSCertificates(c.TLSCertsDir),
		function.Insecure(c.Insecure),
		function.MaxRecvMessageSize(c.MaxRecvMessageSizeMB*1024*1024),
	)
}

func RunCLI() {
	ctx := kong.Parse(&CLI{}, kong.Description("A Crossplane recovery Function."))
	ctx.FatalIfErrorf(ctx.Run())
}
