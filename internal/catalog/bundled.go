package catalog

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
)

//go:embed bundled_seed.json.gz bundled_partner_cards.json.gz
var bundledAssets embed.FS

func readBundledJSON(filename string, dst any) error {
	compressed, err := bundledAssets.ReadFile(filename)
	if err != nil { return fmt.Errorf("read bundled %s: %w", filename, err) }
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil { return fmt.Errorf("gzip bundled %s: %w", filename, err) }
	defer zr.Close()
	b, err := io.ReadAll(io.LimitReader(zr, 4<<20))
	if err != nil { return err }
	if err = json.Unmarshal(b, dst); err != nil { return fmt.Errorf("decode bundled %s: %w", filename, err) }
	return nil
}

func bundledLocations() ([]Location, error) {
	var locations []Location
	err := readBundledJSON("bundled_seed.json.gz", &locations)
	return locations, err
}

func bundledPartnerCards() (map[string]Override, error) {
	cards := map[string]Override{}
	err := readBundledJSON("bundled_partner_cards.json.gz", &cards)
	return cards, err
}
