package twogis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

type Adapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     *slog.Logger
	now     func() time.Time
}

func New(baseURL, apiKey string, log *slog.Logger) *Adapter {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Adapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		log: log,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (a *Adapter) Route(ctx context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	if a.baseURL == "" || a.apiKey == "" {
		return routing.RouteResult{}, routing.ErrProviderUnavailable
	}
	body := map[string]any{
		"points": []map[string]any{
			point(req.Origin),
			point(req.Destination),
		},
		"route_mode":   modeOr(req.RouteMode, routing.RouteFastest),
		"traffic_mode": trafficOr(req.TrafficMode),
	}
	defaultsUsed := !req.VehicleProfile.Complete()
	if truck := truckBody(req.VehicleProfile); truck != nil {
		body["truck"] = truck
		body["transport"] = "truck"
	}
	raw, err := a.post(ctx, "/routing/7.0.0/global", body)
	if err != nil {
		return routing.RouteResult{}, err
	}
	parsed, err := parseRoute(raw)
	if err != nil {
		return routing.RouteResult{}, err
	}
	now := a.now()
	traffic := trafficOr(req.TrafficMode)
	parsed.Provider = "2GIS"
	parsed.CalculatedAt = now
	parsed.TrafficMode = traffic
	parsed.RouteMode = modeOr(req.RouteMode, routing.RouteFastest)
	parsed.RequestFingerprint = routing.Fingerprint("2GIS", req)
	parsed.ProviderDefaultUsed = defaultsUsed
	parsed.ExpiresAt = routing.Expiry(traffic, now)
	return parsed, nil
}

func (a *Adapter) Matrix(ctx context.Context, req routing.MatrixRequest) (routing.MatrixResult, error) {
	if a.baseURL == "" || a.apiKey == "" {
		return routing.MatrixResult{}, routing.ErrProviderUnavailable
	}
	points := make([]map[string]any, 0, len(req.Origins)+len(req.Destinations))
	sources := make([]int, 0, len(req.Origins))
	targets := make([]int, 0, len(req.Destinations))
	for i, origin := range req.Origins {
		sources = append(sources, i)
		points = append(points, latLon(origin))
	}
	base := len(req.Origins)
	for i, dest := range req.Destinations {
		targets = append(targets, base+i)
		points = append(points, latLon(dest))
	}
	body := map[string]any{
		"points": points, "sources": sources, "targets": targets,
	}
	if req.VehicleProfile.Complete() {
		body["transport"] = "truck"
		body["truck"] = truckBody(req.VehicleProfile)
	}
	raw, err := a.post(ctx, "/get_dist_matrix", body)
	if err != nil {
		return routing.MatrixResult{}, err
	}
	cells, err := parseMatrix(raw, len(req.Origins))
	if err != nil {
		return routing.MatrixResult{}, err
	}
	return routing.MatrixResult{
		Provider: "2GIS", Cells: cells, CalculatedAt: a.now(),
		TrafficMode: trafficOr(req.TrafficMode), RouteMode: modeOr(req.RouteMode, routing.RouteFastest),
	}, nil
}

func (a *Adapter) post(ctx context.Context, path string, body map[string]any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, routing.ErrInvalidResponse
	}
	endpoint, err := url.Parse(a.baseURL + path)
	if err != nil {
		return nil, routing.ErrProviderUnavailable
	}
	query := endpoint.Query()
	query.Set("key", a.apiKey)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, routing.ErrProviderUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		a.log.Info("routing request failed", "provider", "2GIS")
		if isTimeout(err) {
			return nil, routing.ErrTimeout
		}
		return nil, routing.ErrProviderUnavailable
	}
	defer resp.Body.Close()
	a.log.Info("routing response", "provider", "2GIS", "status", resp.StatusCode)
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, routing.ErrInvalidResponse
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, routing.ErrRouteNotFound
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, routing.ErrProviderUnavailable
	}
	if resp.StatusCode >= 400 {
		return nil, routing.ErrInvalidResponse
	}
	return raw, nil
}

func parseRoute(raw []byte) (routing.RouteResult, error) {
	var doc struct {
		Status string `json:"status"`
		Result []struct {
			ID            string `json:"id"`
			TotalDistance int    `json:"total_distance"`
			TotalDuration int    `json:"total_duration"`
			Maneuvers     []struct {
				OutcomingPath struct {
					Geometry []struct {
						Selection string `json:"selection"`
					} `json:"geometry"`
				} `json:"outcoming_path"`
			} `json:"maneuvers"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return routing.RouteResult{}, routing.ErrInvalidResponse
	}
	if strings.EqualFold(doc.Status, "FAIL") || len(doc.Result) == 0 {
		return routing.RouteResult{}, routing.ErrRouteNotFound
	}
	item := doc.Result[0]
	if item.TotalDistance < 0 || item.TotalDuration < 0 {
		return routing.RouteResult{}, routing.ErrInvalidResponse
	}
	coords := make([][]float64, 0)
	for _, maneuver := range item.Maneuvers {
		for _, geom := range maneuver.OutcomingPath.Geometry {
			parsed, err := parseLineString(geom.Selection)
			if err != nil {
				return routing.RouteResult{}, routing.ErrInvalidResponse
			}
			coords = append(coords, parsed...)
		}
	}
	if len(coords) == 0 {
		return routing.RouteResult{}, routing.ErrInvalidResponse
	}
	var routeID *string
	if item.ID != "" {
		id := item.ID
		routeID = &id
	}
	return routing.RouteResult{
		ProviderRouteID: routeID,
		DistanceM:       item.TotalDistance,
		DurationSeconds: item.TotalDuration,
		Geometry:        routing.Geometry{Type: "LineString", Coordinates: coords},
	}, nil
}

func parseMatrix(raw []byte, originCount int) ([]routing.MatrixCell, error) {
	var doc struct {
		Routes []struct {
			SourceID int    `json:"source_id"`
			TargetID int    `json:"target_id"`
			Distance int    `json:"distance"`
			Duration int    `json:"duration"`
			Status   string `json:"status"`
		} `json:"routes"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Routes == nil {
		return nil, routing.ErrInvalidResponse
	}
	cells := make([]routing.MatrixCell, 0, len(doc.Routes))
	for _, route := range doc.Routes {
		cell := routing.MatrixCell{OriginIndex: route.SourceID, DestinationIndex: route.TargetID - originCount}
		if !strings.EqualFold(route.Status, "OK") {
			cell.Err = routing.ErrRouteNotFound
		} else if route.Distance < 0 || route.Duration < 0 {
			return nil, routing.ErrInvalidResponse
		} else {
			cell.DistanceM = route.Distance
			cell.DurationSeconds = route.Duration
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

func parseLineString(value string) ([][]float64, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToUpper(value), "LINESTRING(") || !strings.HasSuffix(value, ")") {
		return nil, fmt.Errorf("geometry")
	}
	body := value[len("LINESTRING(") : len(value)-1]
	parts := strings.Split(body, ",")
	out := make([][]float64, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) != 2 {
			return nil, fmt.Errorf("geometry")
		}
		lon, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, err
		}
		lat, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, []float64{lon, lat})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("geometry")
	}
	return out, nil
}

func point(p routing.Point) map[string]any {
	return map[string]any{"type": "stop", "lon": p.Longitude, "lat": p.Latitude}
}

func latLon(p routing.Point) map[string]any {
	return map[string]any{"lon": p.Longitude, "lat": p.Latitude}
}

func truckBody(profile routing.VehicleProfile) map[string]any {
	body := map[string]any{}
	if profile.GrossWeightKg != nil {
		body["mass"] = *profile.GrossWeightKg / 1000
	}
	if profile.HeightM != nil {
		body["height"] = *profile.HeightM
	}
	if profile.WidthM != nil {
		body["width"] = *profile.WidthM
	}
	if profile.LengthM != nil {
		body["length"] = *profile.LengthM
	}
	if profile.AxleLoadKg != nil {
		body["axle_load"] = *profile.AxleLoadKg / 1000
	}
	if profile.DangerousCargo != nil {
		body["dangerous_cargo"] = *profile.DangerousCargo
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func modeOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func trafficOr(value string) string {
	if value == "" {
		return routing.TrafficStatic
	}
	return value
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
