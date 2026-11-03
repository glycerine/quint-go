package quint

import "context"

// RenderErrorContext adds optional context terminal styles to a session error.
// It still returns diagnostic text when the context has been cancelled.
func (s *REPLSession) RenderErrorContext(ctx context.Context, err error) string {
	return s.renderError(err, DiagnosticRenderOptions{HideLineNumbers: true, Decorate: terminalValueDecorator(ctx)})
}
