package spammy

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/pkg/namesgenerator"
	ap "github.com/go-ap/activitypub"
	h "github.com/go-ap/handlers"
)

const (
	Actors h.CollectionType = "actors"
	ServiceAPI ap.IRI = ""
)

var availableExtensions = [...]string{
	// text
	"html",
	"txt",
	"md",
	// document?
	"svg",
	// image
	"jpg",
	"png",
	// audio
	"mp3",
	// video
	"webm",
}

func init() {
	data.Walk("data/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			ff, err := data.Open(path)
			if err != nil {
				return err
			}
			defer ff.Close()

			data, err := ioutil.ReadAll(ff)
			if err != nil {
				return err
			}
			contentType, _, _ := mime.ParseMediaType(http.DetectContentType(data))
			if validRandomFiles[contentType] == nil {
				validRandomFiles[contentType] = make([][]byte, 0)
			}
			validRandomFiles[contentType] = append(validRandomFiles[contentType], data)
		}
		return nil
	})
}

var validRandomFiles = make(map[string][][]byte)

func getObjectTypes(data []byte) (ap.ActivityVocabularyType, ap.MimeType) {
	contentType := http.DetectContentType(data)
	var objectType ap.ActivityVocabularyType

	contentType, _, _ = mime.ParseMediaType(contentType)
	switch contentType {
	case "text/html", "text/markdown", "text/plain":
		objectType = ap.NoteType
		if len(data) > 600 {
			objectType = ap.ArticleType
		}
		if bytes.Contains(data, []byte{'<','s','v','g'}) {
			objectType = ap.DocumentType
			contentType = "image/svg+xml"
		}
	case "image/svg+xml":
		objectType = ap.DocumentType
	case "video/webm":
		fallthrough
	case "video/mp4":
		objectType = ap.VideoType
	case "audio/mp3":
		objectType = ap.AudioType
	case "image/png":
		fallthrough
	case "image/jpg":
		objectType = ap.ImageType
	}
	return objectType, ap.MimeType(contentType)
}

func GetRandomItemFromMap(m map[ap.IRI]ap.Item) ap.Item {
	pos := rand.Int() % len(m)
	cnt := 0
	for _, it := range m {
		if cnt == pos {
			return it
		}
		cnt++
	}
	return nil
}

func getRandomContentByMimeType(mimeType ap.MimeType) []byte {
	if validArray, ok := validRandomFiles[string(mimeType)]; ok {
		return validArray[rand.Int()%len(validArray)]
	}
	return nil
}

func getRandomContent() []byte {
	validArray := make([][]byte, 0)
	for _, files := range validRandomFiles {
		for _, file := range files {
			validArray = append(validArray, file)
		}
	}
	return validArray[rand.Int()%len(validArray)]
}

func getRandomName() []byte {
	return []byte(namesgenerator.GetRandomName(0))
}

func RandomActor(parent ap.Item) ap.Item {
	act := new(ap.Actor)
	act.Name = ap.NaturalLanguageValues{
		{ap.NilLangRef, getRandomName()},
	}
	act.PreferredUsername = act.Name
	act.Type = ap.PersonType
	act.AttributedTo = parent
	act.Icon = RandomImage("image/png", parent)
	return act
}

func RandomImage(mime ap.MimeType, parent ap.Item) ap.Item {
	img := new(ap.Image)
	img.Type = ap.ImageType
	img.MediaType = mime
	img.AttributedTo = parent

	data := getRandomContentByMimeType(mime)
	buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))
	base64.RawStdEncoding.Encode(buf, data)
	img.Content = ap.NaturalLanguageValues{
		{ap.NilLangRef, buf},
	}
	return img
}

func RandomObject(parent ap.Item) ap.Item {
	data := getRandomContent()
	typ, mime := getObjectTypes(data)

	ob := new(ap.Object)
	ob.Type = typ
	ob.MediaType = mime
	ob.AttributedTo = parent

	if !strings.Contains(string(mime), "text") {
		buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))
		base64.RawStdEncoding.Encode(buf, data)
		data = buf
	} else {
		ob.Summary = ap.NaturalLanguageValues{
			{ap.NilLangRef, data[:bytes.Index(data, []byte{'.'})]},
		}
	}
	ob.Content = ap.NaturalLanguageValues{
		{ap.NilLangRef, data},
	}

	return ob
}

var validForObjectActivityTypes = [...]ap.ActivityVocabularyType{
	ap.LikeType,
	ap.DislikeType,
	ap.DeleteType,
	ap.FlagType,
	ap.BlockType,
	ap.FollowType,
}

var validForActivityActivityTypes = [...]ap.ActivityVocabularyType{
	ap.UndoType,
}

var validActivityTypes = append(validForObjectActivityTypes[:], validForActivityActivityTypes[:]...)

func getActivityTypeByObject(ob ap.Item) ap.ActivityVocabularyType {
	if ob != nil {
		return validForObjectActivityTypes[rand.Int()%len(validForObjectActivityTypes)]
	}
	if ap.ActivityTypes.Contains(ob.GetType()) {
		return validForActivityActivityTypes[rand.Int()%len(validForActivityActivityTypes)]
	}
	return validForObjectActivityTypes[rand.Int()%len(validForObjectActivityTypes)]
}

func RandomActivity(ob ap.Item, parent ap.Item) *ap.Activity {
	act := new(ap.Activity)
	act.Type = getActivityTypeByObject(ob)
	if ob != nil {
		act.Object = ob
	}
	act.AttributedTo = parent
	act.To = ap.ItemCollection{parent.GetLink(), ap.PublicNS}

	return act
}
