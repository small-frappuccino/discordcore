import re

def inject_mock(path):
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Look for functions where we need to mock
    mock_code = """	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return nil }
"""

    if path.endswith('bot_runtime_test.go'):
        if 'origOpenBotArikawaState :=' not in content:
            content = content.replace('func TestBotRuntime_InitializationRouting(t *testing.T) {\n', 'func TestBotRuntime_InitializationRouting(t *testing.T) {\n' + mock_code)
    
    elif path.endswith('bot_supervisor_test.go'):
        # For TestSupervisorFaultIsolation
        fault_mock = """	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error {
		token := s.Token()
		if token == "Bot token2" {
			return errors.New("simulated gateway panic in child runtime ID 2")
		} else if token == "Bot token3" {
			return errors.New("HTTP 401 Unauthorized")
		}
		return nil
	}
"""
        if 'origOpenBotArikawaState :=' not in content:
            content = content.replace('func TestSupervisorFaultIsolation(t *testing.T) {\n', 'func TestSupervisorFaultIsolation(t *testing.T) {\n' + fault_mock)
            
            swarm_mock = """	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return nil }
"""
            content = content.replace('func TestSupervisorSwarmTopology(t *testing.T) {\n', 'func TestSupervisorSwarmTopology(t *testing.T) {\n' + swarm_mock)
            content = content.replace('func TestSupervisorConfigChange(t *testing.T) {\n', 'func TestSupervisorConfigChange(t *testing.T) {\n' + swarm_mock)

    elif path.endswith('runner_test.go'):
        # runner_test already mocked newDiscordSession previously, we need to mock openBotArikawaState
        if 'origOpenBotArikawaState :=' not in content:
            content = content.replace('func TestRun_CascadingRollbackFailures(t *testing.T) {\n', 'func TestRun_CascadingRollbackFailures(t *testing.T) {\n' + mock_code)
            content = content.replace('func TestRun_ResourceCleanupOnBootFailure(t *testing.T) {\n', 'func TestRun_ResourceCleanupOnBootFailure(t *testing.T) {\n' + mock_code)

    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)

inject_mock('bot_runtime_test.go')
inject_mock('bot_supervisor_test.go')
inject_mock('runner_test.go')

print("Mocks injected")
