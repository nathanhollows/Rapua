package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Location", "my-location"},
		{"UPPERCASE", "uppercase"},
		{"hello world", "hello-world"},
		{"multiple   spaces", "multiple-spaces"},
		{"Store #1", "store-1"},
		{"50% Off!", "50-off"},
		{"Hello!!!World", "hello-world"},
		{"a-b-c", "a-b-c"},
		{"-leading-and-trailing-", "leading-and-trailing"},
		{"---", ""},
		{"", ""},
		{"   ", ""},
		{"Citrus Collection", "citrus-collection"},
		{"Ōtākou Whakaihu Waka", "otakou-whakaihu-waka"},
		{"Tāwhaki Pou Whenua", "tawhaki-pou-whenua"},
		{"Te Rangihīroa College", "te-rangihiroa-college"},
		{"Café Crème", "cafe-creme"},
		{"Peñíscola", "peniscola"},
		{"Ångström", "angstrom"},
		{"Old Government Buildings", "old-government-buildings"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := models.Slugify(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
