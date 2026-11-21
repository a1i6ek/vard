package spec

import (
	"reflect"
	"slices"
	"testing"
)

func TestJSONFieldsOfAGoStruct(t *testing.T) {
	type embedded struct {
		ID string `json:"id"`
	}
	type answer struct {
		embedded
		Name    string `json:"name"`
		Label   string `json:"label,omitempty"`
		Secret  string `json:"-"`
		private string
	}
	got := JSONFields(reflect.TypeOf(answer{private: ""}))
	if want := []string{"id", "name", "label"}; !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

func TestPropertiesFollowAllOfAndRefs(t *testing.T) {
	// Problem is in common.yaml, which both documents share.
	got, err := Properties("Problem")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"code", "detail", "params", "status", "title", "type"}; !slices.Equal(got, want) {
		t.Errorf("Problem's fields = %v, want %v", got, want)
	}
	if _, err := Properties("Nothing"); err == nil {
		t.Error("a schema the documents do not have was found")
	}
}
