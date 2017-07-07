package controllers

import jsonpatch "github.com/evanphx/json-patch"
import "encoding/json"

func patch(b []byte, v interface{}) error {
	patch, err := jsonpatch.DecodePatch(b)
	if err != nil {
		return err
	}
	doc, err := json.Marshal(v)
	if err != nil {
		return err
	}
	out, err := patch.Apply(doc)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(out, v); err != nil {
		return err
	}
	return nil
}
