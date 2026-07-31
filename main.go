package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Device struct {
	IP    string `json:"ip"`
	Port  string `json:"port"`
	Name  string `json:"name"`
	Model string `json:"model"`
}

type Config struct {
	IPAddress    string   `json:"ip_address"`
	Port         string   `json:"port"`
	ServerPort   string   `json:"server_port"`
	SubnetPrefix string   `json:"subnet_prefix"`
	SelectedIPs  []string `json:"selected_ips"`
	Devices      []Device `json:"devices"`
}

type BrowserApp struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}

var (
	config     Config
	configLock sync.Mutex
	configFile = "config.json"
	adbPath    string

	// TV'lerde yaygın kullanılan bilinen tarayıcı paketleri
	knownBrowsers = []BrowserApp{
		{Name: "Google Chrome", Package: "com.android.chrome"},
		{Name: "TV Bro", Package: "com.phlox.tvwebbrowser"},
		{Name: "Puffin TV", Package: "com.cloudmosa.puffinTV"},
		{Name: "Firefox for TV", Package: "org.mozilla.tv.firefox"},
		{Name: "OpenInet / HiTV Explore", Package: "com.seraphic.openinet.cvte"},
		{Name: "Stark Store Browser / Silk", Package: "com.stark.store"},
		{Name: "JioPages TV", Package: "com.jio.web"},
		{Name: "AOSP WebViewer / HTMLViewer", Package: "com.android.htmlviewer"},
	}
)

func initADBPath() {
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		execDir = "."
	}

	if runtime.GOOS == "windows" {
		adbPath = filepath.Join(execDir, "platform-tools-win64", "adb.exe")
	} else if runtime.GOOS == "darwin" {
		macADB := filepath.Join(execDir, "platform-tools-darwin", "adb")
		if _, err := os.Stat(macADB); err == nil {
			adbPath = macADB
		} else if pathADB, err := exec.LookPath("adb"); err == nil {
			adbPath = pathADB
		} else {
			adbPath = "adb"
		}
	} else {
		adbPath = filepath.Join(execDir, "platform-tools-linux64", "adb")
	}
}

// Dosya adlarındaki boşluk ve özel karakterleri temizler (ADB ve Android Intent uyumu için)
func sanitizeFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]

	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	cleanName := reg.ReplaceAllString(name, "_")

	multiUnderscore := regexp.MustCompile(`_+`)
	cleanName = multiUnderscore.ReplaceAllString(cleanName, "_")

	cleanName = strings.Trim(cleanName, "_")

	if cleanName == "" {
		cleanName = fmt.Sprintf("media_%d", time.Now().UnixNano())
	}

	return cleanName + strings.ToLower(ext)
}

// Izole edilmiş ADB sunucu portu ile ADB komutlarını çalıştırır (Port çakışmalarını önler)
func runADBCommand(args ...string) ([]byte, error) {
	cmd := exec.Command(adbPath, args...)
	cmd.Env = append(os.Environ(), "ANDROID_ADB_SERVER_PORT=5038")
	return cmd.CombinedOutput()
}

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	file, err := os.ReadFile(configFile)
	if err != nil {
		config = Config{IPAddress: "192.168.1.98", Port: "5555", ServerPort: "9468"}
		saveConfigNoLock()
		return
	}
	json.Unmarshal(file, &config)
	if config.ServerPort == "" {
		config.ServerPort = "9468"
		saveConfigNoLock()
	}
}

func saveConfigNoLock() {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configFile, data, 0644)
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func getTargetDevice() string {
	devs := getTargetDevices()
	if len(devs) > 0 {
		return devs[0]
	}
	return "192.168.1.98:5555"
}

func getTargetDevices() []string {
	configLock.Lock()
	defer configLock.Unlock()

	var targets []string
	if len(config.SelectedIPs) > 0 {
		for _, ip := range config.SelectedIPs {
			if ip != "" {
				targets = append(targets, fmt.Sprintf("%s:5555", ip))
			}
		}
	}
	if len(targets) == 0 {
		ip := config.IPAddress
		if ip == "" {
			ip = "192.168.1.98"
		}
		port := config.Port
		if port == "" {
			port = "5555"
		}
		targets = append(targets, fmt.Sprintf("%s:%s", ip, port))
	}
	return targets
}

func runADBCommandOnDevices(args ...string) map[string]string {
	devices := getTargetDevices()
	results := make(map[string]string)
	var wg sync.WaitGroup
	var resLock sync.Mutex

	for _, dev := range devices {
		wg.Add(1)
		go func(targetDev string) {
			defer wg.Done()
			cmdArgs := append([]string{"-s", targetDev}, args...)
			output, err := runADBCommand(cmdArgs...)
			resLock.Lock()
			if err != nil {
				results[targetDev] = fmt.Sprintf("Error: %v - %s", err, string(output))
			} else {
				results[targetDev] = string(output)
			}
			resLock.Unlock()
		}(dev)
	}

	wg.Wait()
	return results
}

func getServerPort() string {
	configLock.Lock()
	defer configLock.Unlock()
	p := config.ServerPort
	if p == "" {
		p = "9468"
	}
	if !strings.HasPrefix(p, ":") {
		p = ":" + p
	}
	return p
}

type SlideshowManager struct {
	sync.Mutex
	IsRunning  bool     `json:"is_running"`
	Interval   int      `json:"interval"`
	Files      []string `json:"files"`
	CurrentIdx int      `json:"current_idx"`
	stopChan   chan struct{}
}

var slideshowMgr SlideshowManager

func stopSlideshowNoLock() {
	if slideshowMgr.IsRunning && slideshowMgr.stopChan != nil {
		close(slideshowMgr.stopChan)
		slideshowMgr.stopChan = nil
	}
	slideshowMgr.IsRunning = false
}

func startSlideshowLoop(intervalSeconds int, mediaFiles []string) {
	slideshowMgr.Lock()
	stopSlideshowNoLock()

	slideshowMgr.IsRunning = true
	slideshowMgr.Interval = intervalSeconds
	slideshowMgr.Files = mediaFiles
	slideshowMgr.CurrentIdx = 0
	slideshowMgr.stopChan = make(chan struct{})
	stopCh := slideshowMgr.stopChan
	slideshowMgr.Unlock()

	go func(ch chan struct{}, delaySec int, files []string) {
		idx := 0

		for {
			select {
			case <-ch:
				fmt.Println("[*] Slayt Gösterisi Arka Plan Döngüsü Durduruldu.")
				return
			default:
				if len(files) == 0 {
					slideshowMgr.Lock()
					slideshowMgr.IsRunning = false
					slideshowMgr.Unlock()
					return
				}

				currentFile := files[idx]
				slideshowMgr.Lock()
				slideshowMgr.CurrentIdx = idx
				slideshowMgr.Unlock()

				ext := strings.ToLower(filepath.Ext(currentFile))
				intentMime := "image/*"
				if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".3gp" {
					intentMime = "video/*"
				}

				tvPath := fmt.Sprintf("/sdcard/Download/TVCast/%s", currentFile)

				targetDevs := getTargetDevices()
				var wg sync.WaitGroup
				for _, dev := range targetDevs {
					wg.Add(1)
					go func(tDev string) {
						defer wg.Done()
						runADBCommand("-s", tDev, "shell", "am", "start",
							"-a", "android.intent.action.VIEW",
							"-d", fmt.Sprintf("file://%s", tvPath),
							"-t", intentMime,
							"--grant-read-uri-permission",
							"-f", "0x10000000",
						)
					}(dev)
				}
				wg.Wait()

				fmt.Printf("[+] Slayt Döngüsü [%d/%d]: %s (%d saniye - %d cihaz)\n", idx+1, len(files), currentFile, delaySec, len(targetDevs))

				select {
				case <-ch:
					fmt.Println("[*] Slayt Gösterisi Durduruldu.")
					return
				case <-time.After(time.Duration(delaySec) * time.Second):
				}

				idx = (idx + 1) % len(files)
			}
		}
	}(stopCh, intervalSeconds, mediaFiles)
}

var (
	installedBrowsersCache []BrowserApp
	lastBrowserCheckTime   time.Time
	browserCacheLock       sync.Mutex
)

func getInstalledBrowsers(targetDevice string) []BrowserApp {
	browserCacheLock.Lock()
	defer browserCacheLock.Unlock()

	if len(installedBrowsersCache) > 0 && time.Since(lastBrowserCheckTime) < 10*time.Minute {
		return installedBrowsersCache
	}

	slideshowMgr.Lock()
	slideshowRunning := slideshowMgr.IsRunning
	slideshowMgr.Unlock()

	if slideshowRunning && len(installedBrowsersCache) > 0 {
		return installedBrowsersCache
	}

	output, err := runADBCommand("-s", targetDevice, "shell", "pm", "list", "packages")
	if err != nil {
		if len(installedBrowsersCache) > 0 {
			return installedBrowsersCache
		}
		return []BrowserApp{{Name: "Varsayılan TV Tarayıcısı", Package: "default"}}
	}

	pkgList := string(output)
	var installed []BrowserApp

	for _, b := range knownBrowsers {
		if strings.Contains(pkgList, b.Package) {
			installed = append(installed, b)
		}
	}
	installed = append(installed, BrowserApp{Name: "Varsayılan TV Tarayıcısı", Package: "default"})

	installedBrowsersCache = installed
	lastBrowserCheckTime = time.Now()
	return installed
}

func getLocalSubnetPrefix() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "192.168.1."
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipStr := ipnet.IP.String()
				if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || strings.HasPrefix(ipStr, "172.") {
					parts := strings.Split(ipStr, ".")
					if len(parts) == 4 {
						return fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2])
					}
				}
			}
		}
	}
	return "192.168.1."
}

func scanNetworkADB(customSubnet string) []Device {
	subnetPrefix := strings.TrimSpace(customSubnet)
	if subnetPrefix == "" {
		configLock.Lock()
		subnetPrefix = config.SubnetPrefix
		configLock.Unlock()
	}
	if subnetPrefix == "" {
		subnetPrefix = getLocalSubnetPrefix()
	}

	subnetPrefix = strings.TrimSuffix(subnetPrefix, "/")
	if !strings.HasSuffix(subnetPrefix, ".") {
		subnetPrefix = subnetPrefix + "."
	}

	configLock.Lock()
	config.SubnetPrefix = subnetPrefix
	saveConfigNoLock()
	configLock.Unlock()

	fmt.Printf("[*] LAN Ağ Taraması Başlatılıyor (Alt Ağ: %s0/24)...\n", subnetPrefix)

	var foundDevices []Device
	var devLock sync.Mutex
	var wg sync.WaitGroup

	for i := 1; i <= 254; i++ {
		wg.Add(1)
		go func(ipNum int) {
			defer wg.Done()
			ip := fmt.Sprintf("%s%d", subnetPrefix, ipNum)
			target := fmt.Sprintf("%s:5555", ip)

			conn, err := net.DialTimeout("tcp", target, 350*time.Millisecond)
			if err == nil {
				conn.Close()
				fmt.Printf("[+] Bulunan ADB Cihaz Portu: %s\n", target)

				// ADB Connect ile bağlan ve cihaz modelini sorgula
				runADBCommand("connect", target)

				modelBytes, _ := runADBCommand("-s", target, "shell", "getprop", "ro.product.model")
				model := strings.TrimSpace(string(modelBytes))

				if model == "" {
					modelBytes, _ = runADBCommand("-s", target, "shell", "getprop", "ro.product.name")
					model = strings.TrimSpace(string(modelBytes))
				}
				if model == "" {
					modelBytes, _ = runADBCommand("-s", target, "shell", "getprop", "net.hostname")
					model = strings.TrimSpace(string(modelBytes))
				}
				if model == "" {
					model = "Android TV Device"
				}

				deviceName := fmt.Sprintf("%s (%s)", model, ip)

				d := Device{
					IP:    ip,
					Port:  "5555",
					Name:  deviceName,
					Model: model,
				}

				devLock.Lock()
				foundDevices = append(foundDevices, d)
				devLock.Unlock()
			}
		}(i)
	}

	wg.Wait()

	fmt.Printf("[+] Ağ Taraması Tamamlandı. Bulunan TV Cihazı Sayısı: %d\n", len(foundDevices))

	// Konfigürasyon ve kayıtlı cihazları güncelle
	configLock.Lock()
	config.Devices = foundDevices
	if len(foundDevices) > 0 {
		if config.IPAddress == "" {
			config.IPAddress = foundDevices[0].IP
			config.Port = foundDevices[0].Port
		}
	}
	saveConfigNoLock()
	configLock.Unlock()

	return foundDevices
}

func main() {
	initADBPath()
	loadConfig()

	// 1. Konfigürasyon Alma
	http.HandleFunc("/get_config", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		configLock.Lock()
		defer configLock.Unlock()
		json.NewEncoder(w).Encode(config)
	})

	// 2. Konfigürasyon Güncelleme
	http.HandleFunc("/set_config", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		ip := r.URL.Query().Get("ip")
		port := r.URL.Query().Get("port")
		srvPort := r.URL.Query().Get("server_port")

		if ip != "" {
			configLock.Lock()
			config.IPAddress = ip
			if port != "" {
				config.Port = port
			}
			if srvPort != "" {
				config.ServerPort = srvPort
			}
			saveConfigNoLock()
			configLock.Unlock()
			w.Write([]byte("OK"))
		} else {
			http.Error(w, "Geçersiz IP", http.StatusBadRequest)
		}
	})

	// 3. TV'de Yüklü Olan Tarayıcıları Tespiti
	http.HandleFunc("/get_browsers", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		targetDevice := getTargetDevice()
		installed := getInstalledBrowsers(targetDevice)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(installed)
	})

	// 4. Genel Intent / URL Gönderimi (YouTube veya Web)
	http.HandleFunc("/launch_intent", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		intentType := r.URL.Query().Get("type") // "youtube" veya "browser"
		targetURL := r.URL.Query().Get("url")
		pkg := r.URL.Query().Get("package")
		fullscreenParam := r.URL.Query().Get("fullscreen")
		isFullscreen := fullscreenParam == "true" || fullscreenParam == "1"

		if targetURL == "" {
			http.Error(w, "URL parametresi eksik", http.StatusBadRequest)
			return
		}

		targetDevs := getTargetDevices()
		var wg sync.WaitGroup
		for _, dev := range targetDevs {
			wg.Add(1)
			go func(tDev string) {
				defer wg.Done()
				var args []string
				if intentType == "youtube" {
					args = []string{"-s", tDev, "shell", "am", "start",
						"-a", "android.intent.action.VIEW",
						"-d", targetURL,
						"-n", "com.google.android.youtube.tv/com.google.android.apps.youtube.tv.activity.ShellActivity",
					}
				} else {
					if pkg != "" && pkg != "default" {
						args = []string{"-s", tDev, "shell", "am", "start",
							"-a", "android.intent.action.VIEW",
							"-d", targetURL,
							pkg,
						}
					} else {
						args = []string{"-s", tDev, "shell", "am", "start",
							"-a", "android.intent.action.VIEW",
							"-d", targetURL,
						}
					}
				}
				runADBCommand(args...)

				if isFullscreen {
					// 1. Android Immersive Fullscreen modu uygula (Status/nav bar gizleme)
					runADBCommand("-s", tDev, "shell", "settings", "put", "global", "policy_control", "immersive.full=*")

					// 2. Tarayıcının açılmasını bekleyip F11 (Keycode 133) gönder
					go func(device string) {
						time.Sleep(800 * time.Millisecond)
						runADBCommand("-s", device, "shell", "input", "keyevent", "133")
					}(tDev)
				}
			}(dev)
		}
		wg.Wait()

		fmt.Printf("[+] Yönlendirme Başarılı (%s - %d TV - Fullscreen: %v): %s\n", intentType, len(targetDevs), isFullscreen, targetURL)
		w.Write([]byte("OK"))
	})

	// 5. Medya (Fotoğraf ve Video) Dosyalarını TV'ye Aktarma ve Oynatma
	http.HandleFunc("/cast_media", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Sadece POST isteği kabul edilir", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseMultipartForm(500 << 20)
		if err != nil {
			http.Error(w, "Form verisi okunamadı: "+err.Error(), http.StatusBadRequest)
			return
		}

		files := r.MultipartForm.File["media"]
		if len(files) == 0 {
			http.Error(w, "Hiçbir medya dosyası yüklenmedi", http.StatusBadRequest)
			return
		}

		uploadsDir := "uploads"
		os.MkdirAll(uploadsDir, 0755)

		type CastResult struct {
			Filename string `json:"filename"`
			Type     string `json:"type"`
			Status   string `json:"status"`
			Error    string `json:"error,omitempty"`
		}

		var results []CastResult

		for idx, fileHeader := range files {
			safeFilename := sanitizeFilename(fileHeader.Filename)

			contentType := fileHeader.Header.Get("Content-Type")
			isImage := strings.HasPrefix(contentType, "image/")
			isVideo := strings.HasPrefix(contentType, "video/")

			if !isImage && !isVideo {
				ext := strings.ToLower(filepath.Ext(safeFilename))
				if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".bmp" {
					isImage = true
				} else if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".3gp" {
					isVideo = true
				}
			}

			if !isImage && !isVideo {
				results = append(results, CastResult{
					Filename: safeFilename,
					Status:   "error",
					Error:    "Geçersiz dosya formatı. Sadece resim ve video kabul edilir.",
				})
				continue
			}

			file, err := fileHeader.Open()
			if err != nil {
				results = append(results, CastResult{
					Filename: safeFilename,
					Status:   "error",
					Error:    "Dosya açılamadı: " + err.Error(),
				})
				continue
			}

			localPath := filepath.Join(uploadsDir, safeFilename)
			dst, err := os.Create(localPath)
			if err != nil {
				file.Close()
				results = append(results, CastResult{
					Filename: safeFilename,
					Status:   "error",
					Error:    "Yerel dosya oluşturulamadı: " + err.Error(),
				})
				continue
			}

			_, err = io.Copy(dst, file)
			file.Close()
			dst.Close()

			if err != nil {
				results = append(results, CastResult{
					Filename: safeFilename,
					Status:   "error",
					Error:    "Kaydetme hatası: " + err.Error(),
				})
				continue
			}

			// ADB Push ile seçili TÜM TV'lere aktar
			tvPath := fmt.Sprintf("/sdcard/Download/TVCast/%s", safeFilename)
			targetDevs := getTargetDevices()
			var pushWg sync.WaitGroup
			for _, dev := range targetDevs {
				pushWg.Add(1)
				go func(tDev string) {
					defer pushWg.Done()
					runADBCommand("-s", tDev, "shell", "mkdir", "-p", "/sdcard/Download/TVCast")
					runADBCommand("-s", tDev, "push", localPath, tvPath)
					runADBCommand("-s", tDev, "shell", "am", "broadcast",
						"-a", "android.intent.action.MEDIA_SCANNER_SCAN_FILE",
						"-d", fmt.Sprintf("file://%s", tvPath),
					)
				}(dev)
			}
			pushWg.Wait()

			intentMime := "image/*"
			mediaKind := "image"
			if isVideo {
				intentMime = "video/*"
				mediaKind = "video"
			}

			// İlk veya tek dosyada, ya da son dosyada TV ekranında görüntüleme tetikle
			if idx == len(files)-1 || len(files) == 1 {
				var viewWg sync.WaitGroup
				for _, dev := range targetDevs {
					viewWg.Add(1)
					go func(tDev string) {
						defer viewWg.Done()
						runADBCommand("-s", tDev, "shell", "am", "start",
							"-a", "android.intent.action.VIEW",
							"-d", fmt.Sprintf("file://%s", tvPath),
							"-t", intentMime,
							"--grant-read-uri-permission",
							"-f", "0x10000000",
						)
					}(dev)
				}
				viewWg.Wait()
			}

			results = append(results, CastResult{
				Filename: safeFilename,
				Type:     mediaKind,
				Status:   "success",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"results": results,
		})
	})

	// 6. TV'deki Yüklü Medyayı Seçerek Oynatma
	http.HandleFunc("/play_media", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Error(w, "Dosya adı eksik", http.StatusBadRequest)
			return
		}

		ext := strings.ToLower(filepath.Ext(filename))
		intentMime := "image/*"
		if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".3gp" {
			intentMime = "video/*"
		}

		targetDevice := getTargetDevice()
		tvPath := fmt.Sprintf("/sdcard/Download/TVCast/%s", filename)

		output, err := runADBCommand("-s", targetDevice, "shell", "am", "start",
			"-a", "android.intent.action.VIEW",
			"-d", fmt.Sprintf("file://%s", tvPath),
			"-t", intentMime,
			"--grant-read-uri-permission",
			"-f", "0x10000000",
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("ADB Intent hatası: %v - %s", err, string(output)), http.StatusInternalServerError)
			return
		}

		fmt.Printf("[+] TV'de Tekil Medya Başlatıldı: %s (%s)\n", filename, intentMime)
		w.Write([]byte("OK"))
	})

	// 7. Tüm Medya Dosyalarını Temizleme (PC ve TV'den)
	http.HandleFunc("/clear_media", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Slayt gösterisini durdur
		slideshowMgr.Lock()
		stopSlideshowNoLock()
		slideshowMgr.Unlock()

		// Local uploads klasörünü temizle
		uploadsDir := "uploads"
		os.RemoveAll(uploadsDir)
		os.MkdirAll(uploadsDir, 0755)

		// TV üzerindeki /sdcard/Download/TVCast/ klasör içeriğini sil
		targetDevice := getTargetDevice()
		runADBCommand("-s", targetDevice, "shell", "rm", "-rf", "/sdcard/Download/TVCast/*")

		fmt.Println("[+] Tüm Medya Dosyaları PC ve TV'den Temizlendi.")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "All media cleared from PC and TV",
		})
	})

	// 8. Tekil Medya Dosyasını Silme (PC ve TV'den)
	http.HandleFunc("/delete_media", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Error(w, "Dosya adı eksik", http.StatusBadRequest)
			return
		}

		// PC yerel dosyasını sil
		localPath := filepath.Join("uploads", filename)
		os.Remove(localPath)

		// TV üzerindeki dosyayı sil
		targetDevice := getTargetDevice()
		tvPath := fmt.Sprintf("/sdcard/Download/TVCast/%s", filename)
		runADBCommand("-s", targetDevice, "shell", "rm", "-f", tvPath)

		fmt.Printf("[+] Tekil Medya Silindi (PC & TV): %s\n", filename)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Media deleted",
		})
	})

	// 9. Önceden Yüklenmiş Medya Dosyalarını Listeleme
	http.HandleFunc("/list_uploaded_media", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		uploadsDir := "uploads"
		entries, err := os.ReadDir(uploadsDir)
		type MediaFileInfo struct {
			Filename string `json:"filename"`
			Type     string `json:"type"`
			Size     int64  `json:"size"`
		}

		var fileList []MediaFileInfo
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					mediaType := "image"
					if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov" || ext == ".webm" || ext == ".3gp" {
						mediaType = "video"
					}
					fileList = append(fileList, MediaFileInfo{
						Filename: entry.Name(),
						Type:     mediaType,
						Size:     info.Size(),
					})
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if fileList == nil {
			fileList = []MediaFileInfo{}
		}
		json.NewEncoder(w).Encode(fileList)
	})

	// 10. Otomatik Slayt Gösterisi Başlatma (Loop / Döngü)
	http.HandleFunc("/start_slideshow", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		intervalStr := r.URL.Query().Get("interval")
		interval := 5
		if intervalStr != "" {
			fmt.Sscanf(intervalStr, "%d", &interval)
		}
		if interval < 2 {
			interval = 2
		}

		uploadsDir := "uploads"
		entries, err := os.ReadDir(uploadsDir)
		var files []string
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					files = append(files, entry.Name())
				}
			}
		}

		if len(files) == 0 {
			http.Error(w, "Yönlendirilecek medya bulunamadı", http.StatusBadRequest)
			return
		}

		startSlideshowLoop(interval, files)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"message":    "Slideshow started",
			"interval":   interval,
			"total_files": len(files),
		})
	})

	// 11. Slayt Gösterisi Durdurma
	http.HandleFunc("/stop_slideshow", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		slideshowMgr.Lock()
		stopSlideshowNoLock()
		slideshowMgr.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Slideshow stopped",
		})
	})

	// 12. Slayt Gösterisi Durum Bilgisi
	http.HandleFunc("/slideshow_status", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		slideshowMgr.Lock()
		defer slideshowMgr.Unlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_running":  slideshowMgr.IsRunning,
			"interval":    slideshowMgr.Interval,
			"current_idx": slideshowMgr.CurrentIdx,
			"total_files": len(slideshowMgr.Files),
		})
	})

	// 13. TV Üzerindeki Tüm Çalışan Uygulamaları Kapatma ve Ana Sayfaya Dönme
	http.HandleFunc("/close_all_apps", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 1. Slayt gösterisini durdur
		slideshowMgr.Lock()
		stopSlideshowNoLock()
		slideshowMgr.Unlock()

		targetDevice := getTargetDevice()

		// 2. TV Ana ekranına dön (HOME Tuşu - KEYCODE 3)
		runADBCommand("-s", targetDevice, "shell", "input", "keyevent", "3")

		// 3. Bilinen ve 3. parti tüm medya/tarayıcı paketlerini force-stop et
		for _, b := range knownBrowsers {
			if b.Package != "default" {
				runADBCommand("-s", targetDevice, "shell", "am", "force-stop", b.Package)
			}
		}

		// Yaygın medya/video galerileri ve oyuncuları force-stop et
		commonMediaPkgs := []string{
			"com.google.android.youtube.tv",
			"com.google.android.youtube",
			"com.android.gallery3d",
			"com.google.android.apps.photos",
			"org.videolan.vlc",
			"com.mxtech.videoplayer.ad",
			"com.mxtech.videoplayer.pro",
		}
		for _, pkg := range commonMediaPkgs {
			runADBCommand("-s", targetDevice, "shell", "am", "force-stop", pkg)
		}

		// 4. TV ana ekranına dönüşü kesinleştir
		runADBCommand("-s", targetDevice, "shell", "input", "keyevent", "3")

		fmt.Println("[+] TV Üzerindeki Tüm Uygulamalar Kapatıldı ve Ana Sayfaya Dönüldü.")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "All TV applications closed and returned to Home screen",
		})
	})

	// 14. Yerel Ağ (LAN) TV Taraması
	http.HandleFunc("/scan_network", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		subnet := r.URL.Query().Get("subnet")
		devices := scanNetworkADB(subnet)

		w.Header().Set("Content-Type", "application/json")
		if devices == nil {
			devices = []Device{}
		}
		json.NewEncoder(w).Encode(devices)
	})

	// 15. Kayıtlı TV Cihazları Listeleme
	http.HandleFunc("/get_devices", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		configLock.Lock()
		devs := config.Devices
		selectedIP := config.IPAddress
		selectedPort := config.Port
		configLock.Unlock()

		if devs == nil {
			devs = []Device{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"devices":       devs,
			"selected_ip":   selectedIP,
			"selected_port": selectedPort,
		})
	})

	// 16. Aktif Hedef TV Cihazı Seçme
	http.HandleFunc("/select_device", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		ip := r.URL.Query().Get("ip")
		port := r.URL.Query().Get("port")
		if port == "" {
			port = "5555"
		}

		if ip != "" {
			configLock.Lock()
			config.IPAddress = ip
			config.Port = port
			saveConfigNoLock()
			configLock.Unlock()

			// Seçilen cihaza ADB connect at
			target := fmt.Sprintf("%s:%s", ip, port)
			runADBCommand("connect", target)

			fmt.Printf("[+] Aktif Hedef TV Değiştirildi: %s\n", target)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":   "ok",
				"selected": target,
			})
		} else {
			http.Error(w, "Geçersiz IP adresi", http.StatusBadRequest)
		}
	})

	// 17. Çoklu Aktif Hedef TV Cihazı Seçme
	http.HandleFunc("/select_devices", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		ipsStr := r.URL.Query().Get("ips")
		var selectedIPs []string

		if ipsStr != "" {
			for _, ip := range strings.Split(ipsStr, ",") {
				cleanIP := strings.TrimSpace(ip)
				if cleanIP != "" {
					selectedIPs = append(selectedIPs, cleanIP)
				}
			}
		}

		configLock.Lock()
		config.SelectedIPs = selectedIPs
		if len(selectedIPs) > 0 {
			config.IPAddress = selectedIPs[0]
			config.Port = "5555"
		}
		saveConfigNoLock()
		configLock.Unlock()

		for _, ip := range selectedIPs {
			target := fmt.Sprintf("%s:5555", ip)
			go runADBCommand("connect", target)
		}

		fmt.Printf("[+] Aktif Hedef TV Cihazları Güncellendi (%d cihaz): %v\n", len(selectedIPs), selectedIPs)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "ok",
			"selected_ips": selectedIPs,
		})
	})

	// Statik yüklenen medya sunucusu (Gerektiğinde HTTP stream için)
	os.MkdirAll("uploads", 0755)
	http.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir("uploads"))))

	listenPort := getServerPort()
	fmt.Printf("[*] ADBCast Android TV Server (Dynamic Context Router) Dinleniyor: http://127.0.0.1%s\n", listenPort)
	log.Fatal(http.ListenAndServe(listenPort, nil))
}