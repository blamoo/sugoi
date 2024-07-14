package main

import (
	"encoding/json"
	"fmt"
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

func (s *StringArray) SetFromTextArea(str string) {
	lines := strings.Split(str, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		*s = append(*s, line)
	}
}

type IdArray []string

func (t *IdArray) UnmarshalJSON(data []byte) error {
	var m map[string]string
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil
	}

	for k, v := range m {
		*t = append(*t, k+"/"+v)
	}

	return nil
}

func (t IdArray) MarshalJSON() ([]byte, error) {
	// return json.Marshal(([]string)(t))
	m := make(map[string]string, len(t))

	for _, v := range t {
		spl := strings.SplitN(v, "/", 2)
		if len(spl) == 2 {
			m[spl[0]] = spl[1]
		}
	}

	return json.Marshal(m)
}
