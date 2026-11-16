// Command hello is a module with both halves: this process, which serves
// one route, and a browser bundle under ui/, which draws on it.
//
// # The route
//
//	GET /hello   the line this module answers with, and who the host says is asking
//
// The path is relative to the mount, so a caller reaches it at
// /api/v1/modules/com.github.aux.prusa3d.prusa-connect-extension-example/hello. The host decides who may call it
// and then makes the call: view_resource on the instance for a read,
// operate_resource for a write. This process checks no permission and holds no
// grant.
//
// # Settings
//
// module.yaml declares every setting this module reads, each with its type, its
// default and one sentence saying what it controls. The host merges the
// operator's config block over those defaults and sends the result on
// plugin.configure before the first request arrives. Nothing here reads an
// environment variable: a setting an operator cannot see in the manifest is a
// setting nobody can find.
//
// # Errors
//
// faults.go lists every error code of this module. The module answers a
// request it cannot serve with one of those codes and its English title. It
// never answers with a sentence alone, because a caller cannot check one.
//
// # Installing it
//
//	sh install.sh
//
// README.md says what that runs and why every change is a new install.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"

	"github.com/prusa3d/prusa-connect/sdk/extension"
	"github.com/prusa3d/prusa-connect/sdk/fault"
	"github.com/prusa3d/prusa-connect/sdk/routesapi"
	"github.com/prusa3d/prusa-connect/sdk/wire"
	"github.com/prusa3d/prusa-connect/sdk/wirerpc"
)

// manifestYAML is module.yaml, built into the binary. The host asks the child
// what it is and refuses one whose name or version differs from the manifest
// it installed (RESOURCE_MANIFEST_DISAGREES), so both are read from the
// manifest itself rather than written a second time here: raising `version`
// in module.yaml is the whole of a release.
//
//go:embed module.yaml
var manifestYAML []byte

// What this child reports when the host asks it what it is.
//
//nolint:gochecknoglobals // read once from the embedded manifest.
var (
	name    = manifestScalar(manifestYAML, "name")
	release = extension.Version(manifestScalar(manifestYAML, "version"))
)

// manifestScalar is the value of a top-level `key: value` line of the
// manifest, without a trailing comment or quotes. It is enough for the two
// scalars above; the host reads the whole manifest with a YAML parser.
func manifestScalar(document []byte, key string) string {
	for _, line := range strings.Split(string(document), "\n") {
		value, found := strings.CutPrefix(line, key+":")
		if !found {
			continue
		}
		if comment := strings.Index(value, " #"); comment >= 0 {
			value = value[:comment]
		}

		return strings.Trim(strings.TrimSpace(value), `"'`)
	}

	return ""
}

// defaultMessage is the line the route answers with when the operator sets
// nothing. It is the default of the `message` setting in module.yaml, and the
// two are the same words on purpose: what the manifest publishes is what this
// code does when nobody overrides it. TestTheManifestPublishesTheCodesDefault
// holds them together.
const defaultMessage = "Hello is serving."

// Settings is what an operator writes under this instance's config block. The
// host merges it over the defaults module.yaml declares and sends the result on
// plugin.configure. A key this struct does not carry refuses the start, so a
// typo is found before a request is.
type Settings struct {
	// Message is the line GET /hello answers with.
	Message string `json:"message"`
}

// held is the settings this module is running on. It is an atomic value because
// plugin.configure is answered on the wire goroutine and the routes are served
// on others.
//
//nolint:gochecknoglobals // the one piece of state this skeleton has.
var held atomic.Pointer[Settings]

// settings is what this module is running on: what the host sent, or the
// defaults this code holds when the host sent nothing.
func settings() Settings {
	if current := held.Load(); current != nil {
		return *current
	}

	return Settings{Message: defaultMessage}
}

// openAPI is the fragment describing the route, with every path relative to
// the mount. The host holds it to routesapi.ParseFragment at launch and serves
// it rebased under the namespace, which is what cmd/generate-client-sdk reads
// to type this module's surface.
//
//go:embed openapi.json
var openAPI []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

// run writes the handshake line and then serves frames until the host closes
// the channel.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	line, err := wire.Handshake{
		Protocol: wire.ProtocolMajor,
		MinMinor: wire.MinMinor,
		MaxMinor: wire.MaxMinor,
		Address:  wire.StdioAddress,
	}.Line()
	if err != nil {
		return fmt.Errorf("render the handshake: %w", err)
	}

	if _, err = os.Stdout.WriteString(line); err != nil {
		return fmt.Errorf("write the handshake: %w", err)
	}

	conn, err := wirerpc.NewConn(stdio{}, wirerpc.Options{
		Role:    wirerpc.RoleChild,
		Methods: methods(),
	})
	if err != nil {
		return fmt.Errorf("build the session: %w", err)
	}

	// The pid is here because the host's log is the only place a person sees
	// this process. Nothing on the wire carries it.
	fmt.Fprintf(os.Stderr, "%s %s is serving on stdio as pid %d\n", name, release, os.Getpid())

	if err = conn.Serve(ctx); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

// methods is the lifecycle set every child answers, plus the one call that
// carries this module's own HTTP surface.
func methods() wirerpc.Methods {
	return wirerpc.Methods{
		wirerpc.MethodInfo: func(context.Context, wirerpc.Call) (any, error) {
			return routesapi.Describe(
				wirerpc.Info{
					Name:     name,
					Version:  release,
					Contract: wirerpc.Contract,
					// Naming the call is what asks the host for the settings. A
					// module that names none is never configured.
					Serves: []wirerpc.Method{wirerpc.MethodConfigure},
				}, openAPI)
		},
		// The settings the host merged from module.yaml's defaults and this
		// instance's config block, sent once before the first request.
		wirerpc.MethodConfigure: wirerpc.Configure(func(_ context.Context, sent Settings) error {
			if sent.Message == "" {
				sent.Message = defaultMessage
			}

			held.Store(&sent)

			return nil
		}),
		wirerpc.MethodShutdown: func(context.Context, wirerpc.Call) (any, error) {
			fmt.Fprintf(os.Stderr, "%s was asked to stop\n", name)

			return nil, nil
		},
		routesapi.MethodServe: routesapi.Serve(routes()),
	}
}

// routes is the surface this module serves, written as any Go program writes
// one. The paths carry no prefix: /hello here is
// /api/v1/modules/com.github.aux.prusa3d.prusa-connect-extension-example/hello to a caller.
func routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		answer(w, http.StatusOK, map[string]any{
			"message": settings().Message,
			// Who the host says is calling. It is the host's answer and not a
			// header this process trusted.
			"principal": routesapi.PrincipalOf(r),
		})
	})

	// Everything else is this module's own refusal. It carries the code from
	// faults.go and names the path, which is what proves the group prefix was
	// taken off before the request arrived.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		refuse(w, r, RouteNotFound.
			New("this module serves no route at "+r.URL.Path).
			With(fault.StringParam("path", r.URL.Path)))
	})

	return mux
}

// answer writes one JSON body, which is every route above.
func answer(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}

// stdio is the pair the host handed over. Close shuts the reading half, which
// is what ends a session whose host has gone quiet.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return os.Stdin.Close() }
