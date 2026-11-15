package server

// setReporterBrowserScreenshotVisibility keeps the report writer's browser MCP
// permission in sync with the screenshot setting. Other agents retain their
// explicit MCP permissions.
func (s *Server) setReporterBrowserScreenshotVisibility(enabled bool) error {
	reporter, err := s.m.pg.GetAgentByKey("reporter")
	if err != nil || reporter == nil {
		return err
	}
	servers, err := s.m.pg.ListMCP()
	if err != nil {
		return err
	}
	for _, server := range servers {
		if server.Name == browserMCPName {
			// Turning screenshots on is an explicit opt-in. Make the browser MCP
			// usable in the shipped root-based container at the same time as the
			// permission toggle, so the next reporter run gets a working browser
			// instead of a deterministic Chromium sandbox error.
			if enabled {
				server.Args = encodeJSON(ensureBrowserMCPArgs(decodeStrSlice(server.Args), true))
				server.Enabled = true
				if _, err := s.m.pg.SaveMCP(server); err != nil {
					return err
				}
			}
			return s.m.pg.ToggleVisibility(reporter.ID, "mcp", server.ID, enabled)
		}
	}
	// The built-in browser MCP may be deleted by an administrator. Keep the
	// setting writable so it takes effect automatically if the server returns.
	return nil
}
