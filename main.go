package main

import (
	"embed"
	"log"
	"os"

	"golang.org/x/sys/windows"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

var appGlobal *application.App
var mainWindow *application.WebviewWindow

func init() {
	application.RegisterEvent[QueueUpdated]("queue:updated")
}

func main() {
	// 单实例检查
	if err := singleInstance("BiliQueueOverlay"); err != nil {
		showError("B站排队助手", "排队助手已经在运行中，请查看任务栏或右下角。")
		os.Exit(0)
	}

	// 加载配置
	if err := loadConfig(); err != nil {
		log.Printf("load config: %v, using defaults", err)
	}
	cfg := getConfig()

	svc := &AppService{roomID: cfg.RoomID}

	app := application.New(application.Options{
		Name:        "B站排队助手",
		Description: "B站直播弹幕排队悬浮窗 — netsysn",
		Icon:        appIconData,
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	appGlobal = app

	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "B站排队助手",
		Width:            300,
		Height:           460,
		MinWidth:         240,
		MinHeight:        200,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
		Windows: application.WindowsWindow{},
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// singleInstance 使用 Windows 命名互斥体实现单实例。
func singleInstance(name string) error {
	mutexName := "Global\\" + name
	handle, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(mutexName))
	if err != nil {
		return err
	}
	// ERROR_ALREADY_EXISTS = 183
	if e, ok := windows.GetLastError().(windows.Errno); ok && e == 183 {
		windows.CloseHandle(handle)
		return os.ErrExist
	}
	return nil
}

// showError 弹出 Windows 消息框。
func showError(title, msg string) {
	windows.MessageBox(0,
		windows.StringToUTF16Ptr(msg),
		windows.StringToUTF16Ptr(title),
		windows.MB_OK|windows.MB_ICONINFORMATION,
	)
}
