package store

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/dshmyz/gracedb/pkg/types"
)

// GeoQuery specifies a geospatial search area.
type GeoQuery struct {
	Lat      float64 // center latitude
	Lon      float64 // center longitude
	RadiusM  float64 // radius in meters
}

// SearchGeo performs geospatial filtering on embeddings with lat/lon metadata.
func (s *BadgerStore) SearchGeo(collectionName string, query GeoQuery, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	results, err := s.vectorSearch(ctx, coll.ID, nil, opts)
	if err != nil {
		return nil, err
	}

	// Filter by distance.
	filtered := make([]types.ScoredEmbedding, 0, len(results))
	for _, r := range results {
		if r.Metadata == nil {
			continue
		}
		latStr, okLat := r.Metadata["lat"]
		lonStr, okLon := r.Metadata["lon"]
		if !okLat || !okLon {
			continue
		}
		lat, errLat := strconv.ParseFloat(strings.TrimSpace(latStr), 64)
		lon, errLon := strconv.ParseFloat(strings.TrimSpace(lonStr), 64)
		if errLat != nil || errLon != nil {
			continue
		}
		dist := haversine(query.Lat, query.Lon, lat, lon)
		if dist <= query.RadiusM {
			// Add distance as metadata for consumers.
			r.Metadata["_geo_distance_m"] = strconv.FormatFloat(dist, 'f', 1, 64)
			filtered = append(filtered, r)
		}
	}

	// Sort by distance.
	sort.Slice(filtered, func(i, j int) bool {
		di := parseGeoDist(filtered[i])
		dj := parseGeoDist(filtered[j])
		return di < dj
	})

	if opts.TopK > 0 && len(filtered) > opts.TopK {
		filtered = filtered[:opts.TopK]
	}

	return filtered, nil
}

// haversine computes the great-circle distance between two points on Earth.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0 // Earth radius in meters
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return r * c
}

func rad(deg float64) float64 {
	return deg * math.Pi / 180.0
}

func parseGeoDist(r types.ScoredEmbedding) float64 {
	if r.Metadata == nil {
		return math.MaxFloat64
	}
	d, err := strconv.ParseFloat(r.Metadata["_geo_distance_m"], 64)
	if err != nil {
		return math.MaxFloat64
	}
	return d
}
