package derpbind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"tailscale.com/tailcfg"
)

const PublicDERPMapURL = "https://controlplane.tailscale.com/derpmap/default"

func FetchMap(ctx context.Context, url string) (*tailcfg.DERPMap, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch derp map: %s", res.Status)
	}

	var dm tailcfg.DERPMap
	if err := json.NewDecoder(res.Body).Decode(&dm); err != nil {
		return nil, err
	}
	return &dm, nil
}
