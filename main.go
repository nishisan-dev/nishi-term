package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	data, err := os.UserCacheDir()
	if err != nil {
		log.Fatal("diretório de usuário indisponível")
	}
	err = wails.Run(&options.App{
		Title: "NishiTerm", Width: 1200, Height: 780, MinWidth: 800, MinHeight: 540,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		OnShutdown:  app.shutdown, Bind: []interface{}{app},
		Windows: &windows.Options{WebviewUserDataPath: filepath.Join(data, "NishiTerm", "WebView2")},
	})
	if err != nil {
		log.Fatal("não foi possível iniciar NishiTerm; verifique o runtime WebView2")
	}
}
