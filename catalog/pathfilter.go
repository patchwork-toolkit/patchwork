package catalog

import (
	"encoding/json"
	"errors"
	"strings"
)

const (
	FOpEquals   = "equals"
	FOpPrefix   = "prefix"
	FOpSuffix   = "suffix"
	FOpContains = "contains"
)

func recursiveMatch(data interface{}, path []string) interface{} {
	// path matched. return the value
	if len(path) == 0 {
		return data
	}

	// match path recursively
	switch data.(type) {
	case map[string]interface{}:
		for k, v := range data.(map[string]interface{}) {
			if k == path[0] {
				// logger.Printf("MAP key: %s, path: %s, value: %v", k, path, v)
				return recursiveMatch(v, path[1:])
			}
		}
	case []interface{}:
		for _, v := range data.([]interface{}) {
			// follow the array's elements
			if _, ok := v.(map[string]interface{})[path[0]]; ok {
				// logger.Printf("ARRAY key: %s, path: %s, value: %v", path[0], path, v)
				return recursiveMatch(v, path)
			}
		}
	default:
		//TODO->logger.Println("Unknown type for", data)
		logger.Println("Unknown type for", data)
	}

	return nil
}

func MatchObject(object interface{}, path []string, op string, value string) (bool, error) {
	var m interface{}
	b, err := json.Marshal(object)
	if err != nil {
		return false, errors.New("Unable to parse object into JSON")
	}
	json.Unmarshal(b, &m)

	// check if the path exists
	v := recursiveMatch(m, path)
	if v == nil {
		return false, nil
	}

	// check the value
	// should be string
	var stringValue string

	switch v.(type) {
	case string:
		stringValue = v.(string)
	default:
		return false, errors.New("recursiveMatch returned a non-string value")
	}

	switch op {
	case FOpEquals:
		if stringValue == value {
			return true, nil
		} else {
			return false, nil
		}
	case FOpPrefix:
		if strings.HasPrefix(stringValue, value) {
			return true, nil
		} else {
			return false, nil
		}
	case FOpSuffix:
		if strings.HasSuffix(stringValue, value) {
			return true, nil
		} else {
			return false, nil
		}
	case FOpContains:
		if strings.Contains(stringValue, value) {
			return true, nil
		} else {
			return false, nil
		}
	}
	return false, errors.New("Unknown filter operation")
}
