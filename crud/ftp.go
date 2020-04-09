package crud

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bejaneps/csv-webapp/auth"

	"github.com/jlaffaye/ftp"
)

// getFTPEntries returns list of entries in a server.
func getFTPEntries(conn *ftp.ServerConn) ([]*ftp.Entry, error) {
	entries, err := conn.List("/")
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func getLatestFTPFile(conn *ftp.ServerConn) (string, error) {
	entries, err := conn.List("/")
	if err != nil {
		return "", err
	}

	latestFile := entries[0]
	for _, v := range entries {
		if v.Time.UnixNano() > latestFile.Time.UnixNano() {
			latestFile = v
		}
	}

	return latestFile.Name, nil
}

// getRangeEntries returns list of files between start and end dates from ftp
func getRangeEntries(start, end time.Time, conn *ftp.ServerConn) ([]*ftp.Entry, error) {
	ftpConn, err := auth.NewFTPConnection()
	if err != nil {
		return nil, err
	}
	defer auth.CloseFTPConnection()

	entries, err := getFTPEntries(ftpConn)
	if err != nil {
		return nil, err
	}

	var rangeFiles []*ftp.Entry
	for _, v := range entries {
		if ok := v.Time.After(start); ok {
			if ok := v.Time.Before(end); ok {
				rangeFiles = append(rangeFiles, v)
			}
		}
	}

	return rangeFiles, nil
}

// createFTPFile gets file from a server & unzips it in a specified folder. Returns the name of created file
func createFTPFile(name, dir string, conn *ftp.ServerConn) (string, error) {
	resp, err := conn.Retr(name)
	if err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}
	defer resp.Close()

	currDir, err := os.Getwd()
	if err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}

	err = os.Chdir(dir)
	if err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}

	f, err := os.Create(name)
	if err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}
	defer f.Close()

	if _, err := io.Copy(f, resp); err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}

	cmd := exec.Command("gunzip", dir+"/"+f.Name())
	if err = cmd.Run(); err != nil {
		os.Remove(f.Name())
		//return "", errors.New("createFTPFile(): " + err.Error())
	}

	log.Printf("[INFO]: unzipped %s file\n", f.Name())

	err = os.Chdir(currDir)
	if err != nil {
		return "", errors.New("createFTPFile(): " + err.Error())
	}

	return strings.TrimSuffix(dir+"/"+f.Name(), ".gz"), nil
}

// DownloadFTPFiles downloads all ftp files, if they are not downloaded yet
func DownloadFTPFiles() error {
	ftpConn, err := auth.NewFTPConnection()
	if err != nil {
		return errors.New("DownloadFTPFiles(): " + err.Error())
	}
	defer auth.CloseFTPConnection()

	currDir, err := os.Getwd()
	if err != nil {
		return errors.New("DownloadFTPFiles(): " + err.Error())
	}

	var files []string
	err = filepath.Walk(currDir+"/files", func(path string, info os.FileInfo, err error) error {
		files = append(files, info.Name()+".gz")
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errors.New("DownloadFTPFiles(): " + err.Error())
	}

	ftpEntries, err := ftpConn.List("/")
	if err != nil {
		return errors.New("DownloadFTPFiles(): " + err.Error())
	}

	for _, v := range ftpEntries {
		//empty file
		if v.Size == 297 {
			continue
		}
		//monthly files
		if len(v.Name) > 38 {
			continue
		}

		if ok := hasEntry(v.Name, files); !ok {
			name, err := createFTPFile(v.Name, currDir+"/files", ftpConn)
			if err != nil {
				return errors.New("DownloadFTPFiles(): " + err.Error())
			}
			log.Printf("[INFO]: file %s has been downloaded\n", name)
		}
	}

	return nil
}
