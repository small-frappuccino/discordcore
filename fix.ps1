$file = 'pkg/discord/logging/monitoring.go'
$content = Get-Content $file
$start = -1
for ($i = 0; $i -lt $content.Length; $i++) {
    if ($content[$i] -match '^func \(ms \*MonitoringService\) dispatchMonitorTaskLocked') {
        $start = $i
        break
    }
}
if ($start -ge 0) {
    $newContent = $content[0..($start-1)] + "func (ms *MonitoringService) dispatchMonitorTaskLocked(runCtx context.Context, taskType string) {" + "	ms.dispatchMonitorTaskWithPayloadLocked(runCtx, task.Task{Type: taskType, Payload: task.EmptyPayload{}})" + "}" + $content[($start+3)..($content.Length-1)]
    $newContent | Set-Content $file
}
