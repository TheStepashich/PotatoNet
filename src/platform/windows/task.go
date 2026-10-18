//go:build windows

package windows

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"

	"potatonet/src/platform/base"
)

// Задание планировщика создаётся один раз — при первом запуске с правами
// администратора. Дальнейшие запуски (ярлык, автозапуск) идут через него, и
// UAC больше не показывается: задание с уровнем HighestAvailable стартует в
// сессии пользователя с полными правами без запроса. Отдельного триггера у
// задания нет — оно запускается по требованию через `schtasks /run`.
const (
	taskName = "PotatoNet"
	// relaunchFlag передаётся заданию, чтобы его процесс никогда повторно не
	// пытался повысить права (защита от зацикливания, если повышение не вышло).
	relaunchFlag = "--no-relaunch"
)

func schtasksCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x08000000,
	}
	return cmd
}

// TaskExists сообщает, есть ли задание, запускающее именно текущий exe. Если
// задание указывает на другой путь (приложение перенесли), возвращается false —
// тогда используется обычное повышение с UAC, а задание будет пересоздано.
func TaskExists() bool {
	out, err := schtasksCmd("/query", "/tn", taskName, "/xml").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(base.CurrentExe()))
}

// RunElevatedTask запускает задание планировщика (без UAC) и сразу возвращает
// управление: сам GUI стартует отдельным, уже привилегированным процессом.
func RunElevatedTask() error {
	out, err := schtasksCmd("/run", "/tn", taskName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks /run: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureElevatedTask создаёт задание планировщика, если его ещё нет (или если
// оно указывает на другой exe). Вызывается только из привилегированного
// процесса: создание задания с HighestAvailable требует прав администратора.
func EnsureElevatedTask(dataDir string) error {
	if TaskExists() {
		return nil
	}
	return createTask(dataDir)
}

func createTask(dataDir string) error {
	f, err := os.CreateTemp("", "potatonet-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if err := writeUTF16File(f, taskXML(base.CurrentExe(), dataDir, currentUser())); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	out, err := schtasksCmd("/create", "/tn", taskName, "/xml", f.Name(), "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks /create: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func currentUser() string {
	domain := os.Getenv("USERDOMAIN")
	if domain == "" {
		domain = os.Getenv("COMPUTERNAME")
	}
	return domain + `\` + os.Getenv("USERNAME")
}

// writeUTF16File пишет XML в кодировке UTF-16LE с BOM — её требует
// `schtasks /create /xml`.
func writeUTF16File(f *os.File, s string) error {
	if _, err := f.Write([]byte{0xFF, 0xFE}); err != nil {
		return err
	}
	u := utf16.Encode([]rune(s))
	buf := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	_, err := f.Write(buf)
	return err
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func taskXML(exe, dataDir, user string) string {
	args := `--data-dir "` + dataDir + `" ` + relaunchFlag
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>PotatoNet — запуск с правами администратора без запроса UAC.</Description>
  </RegistrationInfo>
  <Triggers />
  <Principals>
    <Principal id="Author">
      <UserId>` + xmlEscape(user) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>Parallel</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + xmlEscape(exe) + `</Command>
      <Arguments>` + xmlEscape(args) + `</Arguments>
    </Exec>
  </Actions>
</Task>`
}
