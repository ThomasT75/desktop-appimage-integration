## Desktop Appimage Integration
Extracts the .desktop file inside the appimage and changes the Exec path to point to the appimage absolute path and moves the .desktop file to `$HOME/.local/share/applications`so the system can see it.
it does the same for icons but it places them into `$HOME/.local/share/icons/appimages/*.png` with the wildcard (*) meaning the icon name it extracted.

## Usage
run the `binary` or the `run.sh` script and point it to the directory of your appimages it will then search the directory for appimages inside that directory meaning no recursive search

## Building
build it like this:
```
$ go build ./main.go
```

## Edge Cases
 - if more than 1 `*.png` gets extracted it will not be able to handle
 - if more than 1 `*.desktop` gets extracted it will not be able to handle
 - some appimages might not be compatible with this
