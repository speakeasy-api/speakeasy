package markdown_test

import (
	"strings"
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/markdown"
	"github.com/stretchr/testify/assert"
)

func TestCreateMarkdownTable(t *testing.T) {
	type args struct {
		contents [][]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Simple table with matching number of columns",
			args: args{
				contents: [][]string{
					{
						"Name", "Age", "Gender",
					},
					{
						"John", "21", "Male",
					},
					{
						"Jane", "20", "Female",
					},
				},
			},
			want: `
| Name   | Age    | Gender |
| ------ | ------ | ------ |
| John   | 21     | Male   |
| Jane   | 20     | Female |`,
		},
		{
			name: "Table with mismatched number of columns",
			args: args{
				contents: [][]string{
					{
						"Name", "Age", "Gender",
					},
					{
						"John",
					},
					{
						"Jane", "21",
					},
				},
			},
			want: `
| Name   | Age    | Gender |
| ------ | ------ | ------ |
| John   |        |        |
| Jane   | 21     |        |`,
		},
		{
			name: "Table with cells less than 3 characters",
			args: args{
				contents: [][]string{
					{
						"N", "A", "G",
					},
					{
						"Jo", "20", "M",
					},
					{
						"Mi", "21", "F",
					},
				},
			},
			want: `
| N   | A   | G   |
| --- | --- | --- |
| Jo  | 20  | M   |
| Mi  | 21  | F   |`,
		},
		{
			name: "Simple table with pipes escaped",
			args: args{
				contents: [][]string{
					{
						"Name", "Age", "Gender",
					},
					{
						"Jo|hn", "21", "|Male",
					},
					{
						"Jane|", "20", "Female",
					},
				},
			},
			want: `
| Name   | Age    | Gender |
| ------ | ------ | ------ |
| Jo\|hn | 21     | \|Male |
| Jane\| | 20     | Female |`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdown.CreateMarkdownTable(tt.args.contents)
			assert.Equal(t, strings.TrimPrefix(tt.want, "\n"), got)
		})
	}
}
