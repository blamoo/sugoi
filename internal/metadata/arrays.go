package metadata

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type StringArray []string

func (s *StringArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = StringArray{}
		return nil
	}

	var err error
	var asString string
	err = json.Unmarshal(data, &asString)

	if err == nil {
		*s = []string{asString}
		return nil
	}

	var asArray []string
	err = json.Unmarshal(data, &asArray)

	if err == nil {
		*s = asArray
		return nil
	}

	return fmt.Errorf("data should be string or string array")
}

// SetFromTextArea replaces the contents with one entry per non-empty line.
func (s *StringArray) SetFromTextArea(str string) {
	*s = nil
	lines := strings.SplitSeq(str, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		*s = append(*s, line)
	}
}

// IdArray holds source ids such as "/item/1970/abc123". It reads both the
// array written by the importer (theplan) and the older {"source": "id"}
// object, whose entries become "source/id". It is always written as an
// array, so a read-then-save never changes or drops the ids.
type IdArray []string

func (t *IdArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = nil
		return nil
	}

	var asArray []string
	if err := json.Unmarshal(data, &asArray); err == nil {
		*t = asArray
		return nil
	}

	var asMap map[string]string
	if err := json.Unmarshal(data, &asMap); err == nil {
		keys := make([]string, 0, len(asMap))
		for k := range asMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		*t = nil
		for _, k := range keys {
			*t = append(*t, k+"/"+asMap[k])
		}
		return nil
	}

	return fmt.Errorf("id_source should be a string array or an object")
}

func (t IdArray) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(t))
}
