package pprof_test

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/ggsrc/gglib/pprof"
)

func Example() {
	// In a service, NewWithDefaultEnvPrefix reads DEBUG_PPROF_*; the server
	// stays off unless DEBUG_PPROF_ENABLED=true.
	srv := pprof.NewWithOptions(pprof.WithAddr("127.0.0.1:0"), pprof.WithToken("secret"))
	if err := srv.Start(context.Background()); err != nil {
		panic(err)
	}
	defer srv.Stop(context.Background())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+srv.Addr().String()+"/debug/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Print(string(body))
	// Output: ok
}
