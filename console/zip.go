package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// ~!zip
// @Alias archive
// @Hidden
// Zips all attachments and embeds in the channel.
// This command is hidden because the zip file is invariably so big it can't be uploaded.
func archive(channel *discordgo.Channel, _ *discordgo.Guild, count int) {
	type FileInfo struct {
		Filename  string
		URL       string
		Timestamp time.Time
	}
	files := make([]FileInfo, 0, 500)
	lastMsg := ""
	for {
		toProc, err := client.ChannelMessages(channel.ID, 100, lastMsg, "", "")
		if err != nil {
			fmt.Println(err)
			input.ReadBytes('\n')
			return
		}
		if count >= 0 && len(toProc) > count {
			toProc = toProc[:count-1]
		}
		if len(toProc) == 0 {
			break
		}
		count -= len(toProc)
		lastMsg = toProc[len(toProc)-1].ID
		for _, v := range toProc {
			ts := v.Timestamp
			for _, a := range v.Attachments {
				files = append(files, FileInfo{a.ID + "-" + a.Filename, a.URL, ts})
			}
			for _, a := range v.Embeds {
				if a.Image != nil && a.Image.URL != "" {
					s := strings.Split(a.Image.URL, "/")
					files = append(files, FileInfo{v.ID + "-" + s[len(s)-1], a.Image.URL, ts})
				}
			}
		}
	}
	fmt.Println(fmt.Sprintf("Found %d messages, zipping...", len(files)))
	fName := fmt.Sprintf("%s/jlort-jlort-%d.zip", os.TempDir(), time.Now().Unix())
	f, err := os.OpenFile(fName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fmt.Printf("Failed to open tempfile %s: %s", fName, err.Error())
		input.ReadBytes('\n')
		return
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	zWriter := zip.NewWriter(buf)
	for _, fInfo := range files {
		header := new(zip.FileHeader)
		header.Name = fInfo.Filename
		header.Modified = fInfo.Timestamp
		s := strings.Split(fInfo.Filename, ".")
		switch s[len(s)-1] {
		case "png", "jpg", "mp3", "ogg", "zip", "7z", "gz", "m4a", "pdf", "docx", "mp4", "mov":
			//Do not compress, default Store
		default:
			header.Method = 8 // Deflate
		}
		fWriter, err := zWriter.CreateHeader(header)
		if err != nil {
			fmt.Printf("Failed to append to zip: %s", err.Error())
			input.ReadBytes('\n')
			return
		}
		resp, err := client.Client.Get(fInfo.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}
		_, err = io.Copy(fWriter, resp.Body)
		if err != nil {
			fmt.Printf("Failed to append to zip: %s", err.Error())
			input.ReadBytes('\n')
			return
		}
	}
	checkFatal(zWriter.Close())
	checkFatal(buf.Flush())
	checkFatal(f.Close())
	fmt.Println("Zip complete! Look for " + fName)
	input.ReadBytes('\n')
}