package pazuzu

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/docker/builder/dockerfile/parser"

	"github.com/zalando-incubator/pazuzu/shared"
)

var ErrInvalidCopyCmdSyntax = fmt.Errorf("Invalid 'COPY' command syntax")

type DockerfileWriter struct {
	buf *bytes.Buffer
}

func NewDockerfileWriter() *DockerfileWriter {
	var buf bytes.Buffer
	return &DockerfileWriter{buf: &buf}
}

func (c *DockerfileWriter) AppendRaw(chunk string) error {
	_, err := c.buf.WriteString(chunk + "\n")
	if err != nil {
		return err
	}

	return nil
}

func fixCopyCmd(node *parser.Node, feature shared.Feature) (string, error) {
	srcNode := node.Next
	if srcNode == nil {
		return "", ErrInvalidCopyCmdSyntax
	}
	dstNode := srcNode.Next
	if dstNode == nil {
		return "", ErrInvalidCopyCmdSyntax
	}

	fixedCmd := fmt.Sprintf("COPY %s/%s %s", feature.Meta.Name, srcNode.Value, dstNode.Value)

	return fixedCmd, nil
}

func (c *DockerfileWriter) AppendFeature(feature shared.Feature) error {
	d := parser.Directive{LookingForDirectives: true}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &d)

	ast, err := parser.Parse(strings.NewReader(feature.Snippet), &d)
	if err != nil {
		return err
	}

	for _, cmdNode := range ast.Children {
		if cmdNode.Value == "copy" {
			fixedCmd, err := fixCopyCmd(cmdNode, feature)
			if err != nil {
				return err
			}
			c.AppendRaw(fixedCmd)
		} else {
			c.AppendRaw(cmdNode.Original)
		}
	}

	return nil
}

func (c *DockerfileWriter) Bytes() []byte {
	return c.buf.Bytes()
}
