package main

import (
	"bufio"
	_ "embed"
	"errors"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var DESKTOP_FILES_DIR = filepath.Join(os.Getenv("HOME"), ".local/share/applications/")
var ICON_FILES_DIR = filepath.Join(os.Getenv("HOME"), ".local/share/icons/appimages/")
var CONFIG_MENU_STRUCTURES = filepath.Join(os.Getenv("HOME"), ".config/menus/applications-merged")
var MENU_DIRECTORY_DIR = filepath.Join(os.Getenv("HOME"), ".local/share/desktop-directories")
var FORCE_OVERWRITE = false
var CREATE_MENU = false

//go:embed appimage_icon.svg
var appIcon []byte
var appIconName = "appimage_icon.svg"

func main()  {
  // CLI flag def
  // verbose := flag.Bool("v", false, "verbose output")
  flag.BoolVar(&CREATE_MENU, "create-menu", CREATE_MENU, "allows the creation of a menu in the app laucher")
  flag.BoolVar(&FORCE_OVERWRITE, "f", FORCE_OVERWRITE, "use to force overwrite files to destination")
  flag.Parse()

  // make needed dirs if it doesn't exist
  os.MkdirAll(DESKTOP_FILES_DIR, 0755)
  os.MkdirAll(ICON_FILES_DIR, 0755)
  os.MkdirAll(CONFIG_MENU_STRUCTURES, 0755)
  os.MkdirAll(MENU_DIRECTORY_DIR, 0755)

  // Get Directory
  appImageDir, err := filepath.Abs(flag.Arg(0))
  if err != nil {
    panic(err.Error())
  }
  
  // Get AppImages
  var appImagesPaths []string
  filepath.WalkDir(appImageDir, func(path string, d fs.DirEntry, err error) error {
    if err != nil {
      return err
    }
    if !d.IsDir() {
      if filepath.Ext(path) == ".AppImage" {
        println(path)
        appImagesPaths = append(appImagesPaths, path)
      }
    } else {
      if appImageDir != path {
        return filepath.SkipDir
      }
    }
    return nil
  })

  if len(appImagesPaths) == 0 {
    println("no appimages found")
  }

  // create the menu
  if CREATE_MENU {
    err := createMenu()
    if err != nil {
      println(err.Error())
      println("failed to create menu")
    }
  }

  // Create .desktop
  for _, app := range appImagesPaths {
    err1 := createDesktopFile(app)
    err2 := createIcon(app)
    if err1 != nil {
      println(err1.Error())
      os.Exit(0)
    }
    if err2 != nil {
      println(err2.Error())
      os.Exit(0)
    }

  }

  cmd := exec.Command("kbuildsycoca5", "--noincremental")
  err = cmd.Run()
  if err != nil {
    println(err)
    os.Exit(-1)
  }

  err = os.Remove("squashfs-root")
  if err != nil && !errors.Is(err, fs.ErrNotExist) {
    println(err)
    os.Exit(-1)
  }
}

func fileExist(path string) bool {
  _, err := os.Stat(path) 
  return errors.Is(err, fs.ErrExist) || err == nil
}

func MoveFile(oldpath, newpath string, force bool) error {
  _, err := os.Stat(newpath) 
  if os.IsExist(err) {
    if force {
      return os.Rename(oldpath, newpath)
    } else {
      return errors.New("file already exist (movefile)")
    }
  } else {
    return os.Rename(oldpath, newpath)
  }
}

func CopyFile(src, dst string) error {
  var err error
  if fileExist(src) {
    // open src
    var sFile *os.File
    sFile, err = os.Open(src)
    if err != nil {
      return err
    }
    defer sFile.Close()

    // create dst
    var dFile *os.File
    dFile, err = os.Create(dst)
    if err != nil {
      return err
    }
    defer dFile.Close()

    // read src to dst
    _, err = dFile.ReadFrom(sFile)
    if err != nil {
      return err
    }
  } else {
    return errors.New("src file doesn't exist: "+src)
  }
  return nil
}

func extractFromAppImage(appimagePath string, pattern string) *exec.Cmd {
  cmd := exec.Command(appimagePath, "--appimage-extract", pattern)
  cmd.Stdout = os.Stdout
  cmd.Stdin = os.Stdin
  cmd.Stderr = os.Stderr
  return cmd
} 

func createIcon(absPathToAppImage string) error {
  // extract file using appimage
  cmd := extractFromAppImage(absPathToAppImage, "*.png")
  cmd.Run()

  // find the extracted file
  m, err := filepath.Glob("squashfs-root/*.png")
  if err != nil {
    panic(err)
  }

  // edge case
  if len(m) > 1 {
    return errors.New("more than 1 icon extracted")
  }

  // var defs
  iconExtracted := m[0]
  iconDestination := filepath.Join(ICON_FILES_DIR, filepath.Base(m[0]))

  // don't overwrite by default
  if fileExist(iconDestination) && !FORCE_OVERWRITE {
    return errors.New("this icon already exist")
  }

  // copy png to icons folder
  err = CopyFile(iconExtracted, iconDestination)
  if err != nil {
    panic(err)
  }

  // trash the extracted
  err = os.Remove(iconExtracted)
  if err != nil {
    panic(err)
  }

  return nil
}

func createDesktopFile(absPathToAppImage string) error {
  // use the appimage to extract the file for us
  cmd := extractFromAppImage(absPathToAppImage, "*.desktop")
  cmd.Run()

  // get the dekstop file
  m, err := filepath.Glob("squashfs-root/*.desktop")
  if err != nil {
    panic(err)
  }

  // don't know what to do
  if len(m) > 1 {
    return errors.New("more than 1 dekstop file extracted")
  }

  // var defs
  desktopExtracted := m[0]
  desktopDestination := filepath.Join(DESKTOP_FILES_DIR, filepath.Base(m[0]))

  // don't overwrite by default
  if fileExist(desktopDestination) && !FORCE_OVERWRITE {
    return errors.New("this desktop file was already created")
  }

  // open extracted for reading
  f, err := os.Open(desktopExtracted)
  if err != nil {
    panic(err)
  }
  defer f.Close()

  // open destination for writing
  fOut, err := os.Create(desktopDestination)
  if err != nil {
    panic(err)
  }
  defer fOut.Close()

  // create a scanner from extracted
  fScanner := bufio.NewScanner(f)

  // scan line by line
  for fScanner.Scan() {
    line := fScanner.Text()
    newLine := line
    // replace the relative path with an absolute path
    if strings.HasPrefix(line, "Exec=") {
      _, after, _ := strings.Cut(line, " ")
      // add quotes for paths with spaces
      newLine = "Exec='"+absPathToAppImage+"' "+after
    }
    // add this to the menu category
    if strings.HasPrefix(line, "Categories=") {
      newLine = newLine + "AppImage;"
    }

    // add folder to icon path 
    if strings.HasPrefix(line, "Icon=") {
      _, after, _ := strings.Cut(line, "=")
      newLine = "Icon="+ICON_FILES_DIR+"/"+after+".png"
    }

    fOut.WriteString(newLine+"\n")
  }

  // trash the extracted
  err = os.Remove(desktopExtracted)
  if err != nil {
    panic(err)
  }

  return nil
}

func createMenu() error {
  // the directory "file"
  menuDirectory :=
`[Desktop Entry]
Type=Directory
Name=AppImages
Icon=%%ICON%%
`

  // the structure "file"
  menuStructure := 
`<!DOCTYPE Menu PUBLIC "-//freedesktop//DTD Menu 1.0//EN"
"http://www.freedesktop.org/standards/menu-spec/menu-1.0.dtd">
<Menu>
    <Name>Applications</Name>
    <Menu>
        <Name>AppImages</Name>
        <Directory>appimages.directory</Directory>
        <Include>
            <Category>AppImage</Category>
        </Include>
    </Menu>
</Menu>`

  // copy menu icon to place
  // might be a bad idea to copy to the same place as the appimages
  // but because of the "unique" name im leaving it as is
  appIconDst := filepath.Join(ICON_FILES_DIR, appIconName)
  if fileExist(appIconDst) && !FORCE_OVERWRITE {
    return errors.New("menu icon file was already created")
  }
  iconFile, err := os.Create(appIconDst)
  defer iconFile.Close()
  if err != nil {
    panic(err)
  }
  _, err = iconFile.Write(appIcon)
  if err != nil {
    panic(err)
  }

  menuDirectory = strings.Replace(menuDirectory, "%%ICON%%", appIconDst, 1)

  // menu directory
  appMenuDirectory := filepath.Join(MENU_DIRECTORY_DIR, "appimages.directory")
  // don't overwrite by default
  if fileExist(appMenuDirectory) && !FORCE_OVERWRITE {
    return errors.New("this menu directory file was already created")
  }
  directoryFile, err := os.Create(appMenuDirectory)
  if err != nil {
    panic(err)
  }
  defer directoryFile.Close()
  _, err = directoryFile.WriteString(menuDirectory)
  if err != nil {
    panic(err)
  }

  // menu structure
  appMenuStructure := filepath.Join(CONFIG_MENU_STRUCTURES, "appimages.menu")
  // don't overwrite by default
  if fileExist(appMenuStructure) && !FORCE_OVERWRITE {
    return errors.New("this menu structure file was already created")
  }
  structureFile, err := os.Create(appMenuStructure)
  if err != nil {
    panic(err)
  }
  defer structureFile.Close()
  _, err = structureFile.WriteString(menuStructure)
  if err != nil {
    panic(err)
  }
  

  return nil
}
