package api

import (
	"context"
	"net/http"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
	"github.com/alpha-liu-01/rayut/daemon/internal/route"
	"github.com/alpha-liu-01/rayut/daemon/internal/traffic"
)

func (s *Server) trafficView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/traffic" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if s.traffic == nil {
		writeJSON(w, traffic.View{Samples: []traffic.Sample{}})
		return
	}
	writeJSON(w, s.traffic.View())
}

func (s *Server) watchTraffic(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.sampleTraffic()
		}
	}
}

func (s *Server) sampleTraffic() {
	if s.traffic == nil {
		return
	}
	if _, ok := core.Alive(); !ok {
		s.traffic.Account(traffic.Sample{}, 0, 0)
		return
	}
	_ = route.PinIPv6Gateways()
	snap, err := s.readSessionTraffic(context.Background())
	if err != nil {
		return
	}
	s.traffic.Account(traffic.Sample{Up: snap.Up, Down: snap.Down}, snap.UploadTotal, snap.DownloadTotal)
}

func (s *Server) readSessionTraffic(ctx context.Context) (mihomoapi.Traffic, error) {
	client, err := mihomoapi.Default()
	if err != nil {
		return mihomoapi.Traffic{}, err
	}
	return client.SessionTraffic(ctx)
}
