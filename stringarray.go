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
