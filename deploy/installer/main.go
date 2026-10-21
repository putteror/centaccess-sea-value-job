package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	installDir  = `C:\Program Files\CentAccess\SeaValueJob`
	taskName    = "SeaValueJobService"
	exeName     = "sea-value-job.exe"
	downloadURL = "https://storage.centaccess.com/releases/custom/sea-value-job/latest/sea-value-job-windows-amd64.exe"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("  Sea Value Job Service - Windows Installer")
	fmt.Println("==================================================")

	// 1. ตรวจสอบสิทธิ์ Administrator
	if !isAdmin() {
		fmt.Println("[INFO] Requesting Administrator privileges...")
		if err := runAsAdmin(); err != nil {
			fmt.Printf("[ERROR] Failed to elevate permissions: %v\n", err)
			waitForExit(1)
		}
		return
	}

	destExe := filepath.Join(installDir, exeName)
	envFile := filepath.Join(installDir, ".env")

	// 2. สร้างโฟลเดอร์สำหรับติดตั้ง
	fmt.Printf("[INFO] Creating installation directory at %s...\n", installDir)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Printf("[ERROR] Failed to create installation directory: %v\n", err)
		waitForExit(1)
	}

	// 3. หยุด Task และ Process เดิมถ้ากำลังทำงานอยู่
	fmt.Println("[INFO] Stopping any existing Sea Value Job processes...")
	_ = exec.Command("schtasks", "/end", "/tn", taskName).Run()
	_ = exec.Command("taskkill", "/F", "/IM", exeName).Run()
	time.Sleep(500 * time.Millisecond)

	// 4. ติดตั้งไฟล์ Binary (จากไฟล์ในโฟลเดอร์เดียวกัน หรือดาวน์โหลดจาก R2)
	localBinaries := []string{
		"sea-value-job-windows-amd64.exe",
		"sea-value-job.exe",
	}

	installed := false
	for _, bin := range localBinaries {
		if _, err := os.Stat(bin); err == nil {
			fmt.Printf("[INFO] Found local binary: %s. Copying to %s...\n", bin, destExe)
			if err := copyFile(bin, destExe); err == nil {
				installed = true
				break
			}
		}
	}

	if !installed {
		fmt.Printf("[INFO] Local binary not found. Downloading from %s...\n", downloadURL)
		if err := downloadFile(destExe, downloadURL); err != nil {
			fmt.Printf("[ERROR] Failed to download binary: %v\n", err)
			waitForExit(1)
		}
		fmt.Println("[SUCCESS] Downloaded binary successfully.")
	}

	// 5. สร้างไฟล์ .env เริ่มต้น (ถ้ายังไม่มี)
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		fmt.Println("[INFO] Generating default .env configuration...")
		envContent := `# CentAccess API Endpoints
APP_API_URL=http://localhost:3001
APP_OVERTIME_URL=http://localhost:3001

# Authentication: Option A - Static JWT Bearer Token / API Key
APP_API_KEY=

# Authentication: Option B - Username / Password for Automated JWT Login & Refresh
APP_USERNAME=sea_value_admin
APP_PASSWORD=P@ssw0rd!234

# HTTP Web UI & Management Port
HTTP_PORT=9090

# Optional: Custom paths for config and devices JSON files
CONFIG_FILE_PATH=./config.json
DEVICES_FILE_PATH=./devices.json
`
		if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
			fmt.Printf("[ERROR] Failed to create .env file: %v\n", err)
			waitForExit(1)
		}
		fmt.Println("[SUCCESS] Generated default .env file.")
	} else {
		fmt.Println("[INFO] Existing .env file found. Skipping overwrite.")
	}

	// 6. สร้างไฟล์ run-manual.bat สำหรับเทสท์รัน
	manualBat := filepath.Join(installDir, "run-manual.bat")
	batContent := fmt.Sprintf("@echo off\r\ncd /d \"%s\"\r\n%s\r\npause\r\n", installDir, exeName)
	_ = os.WriteFile(manualBat, []byte(batContent), 0644)

	// 7. ลงทะเบียน Task Scheduler (Auto-start on Boot under SYSTEM account พร้อมตั้งค่า Start In)
	fmt.Printf("[INFO] Registering Windows Scheduled Task: %s (Auto-start on Boot)...\n", taskName)
	_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

	// ใช้ PowerShell Script แบบ inline เพื่อกำหนด WorkingDirectory (Start In)
	psCmdText := fmt.Sprintf(`$action = New-ScheduledTaskAction -Execute '%s' -WorkingDirectory '%s'; $trigger = New-ScheduledTaskTrigger -AtStartup; $principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Days 0); Register-ScheduledTask -TaskName '%s' -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force`, destExe, installDir, taskName)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmdText)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[WARNING] PowerShell Task registration output: %s (Error: %v)\n", string(out), err)
		// Fallback XML approach เพื่อการันตี WorkingDirectory
		xmlTemplate := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
    <BootTrigger>
      <Enabled>true</Enabled>
    </BootTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>S-1-5-18</UserId>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
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
      <Command>%s</Command>
      <WorkingDirectory>%s</WorkingDirectory>
    </Exec>
  </Actions>
</Task>`, destExe, installDir)
		xmlFile := filepath.Join(installDir, "task.xml")
		_ = os.WriteFile(xmlFile, []byte(xmlTemplate), 0644)
		xmlCmd := exec.Command("schtasks", "/create", "/tn", taskName, "/xml", xmlFile, "/f")
		_ = xmlCmd.Run()
		_ = os.Remove(xmlFile)
	}
	fmt.Println("[SUCCESS] Registered Windows Scheduled Task with 'Start in' (WorkingDirectory) configured.")

	// 8. เริ่มต้น Service ทันที
	fmt.Printf("[INFO] Starting background task %s...\n", taskName)
	_ = exec.Command("schtasks", "/run", "/tn", taskName).Run()

	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("  🎉 Sea Value Job Service Installed Successfully!")
	fmt.Println("==================================================")
	fmt.Printf("• Install Path   : %s\n", installDir)
	fmt.Printf("• Config File    : %s\n", envFile)
	fmt.Printf("• Web Portal     : http://localhost:9090\n")
	fmt.Printf("• Background Task: %s (Runs automatically on boot)\n", taskName)
	fmt.Printf("• Start Command  : schtasks /run /tn %s\n", taskName)
	fmt.Printf("• Stop Command   : schtasks /end /tn %s\n", taskName)
	fmt.Println("==================================================")

	waitForExit(0)
}

func isAdmin() bool {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

func runAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	psCmd := fmt.Sprintf("Start-Process -FilePath '%s' -Verb RunAs", exePath)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	return cmd.Start()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func waitForExit(code int) {
	fmt.Println("\nPress Enter to exit...")
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
	os.Exit(code)
}
