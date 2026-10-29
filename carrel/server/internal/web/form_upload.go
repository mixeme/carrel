package web

import (
	"bytes"
	"context"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/photo"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// formUpload lives for one Save. Small files share one bounded inline
// budget; larger files go straight to the attachment folder. No copy is
// made on Carrel's disk. Until the record is written, every new remote
// file belongs to this request and is taken back on any refusal.
type formUpload struct {
	files  []attachFile
	photo  []byte
	placed []*placedFile
	chosen bool
	saved  bool
	notice *view.Notice
}

type formUploadKey struct{}

func formUploadOf(r *http.Request) *formUpload {
	if u, ok := r.Context().Value(formUploadKey{}).(*formUpload); ok {
		return u
	}
	return &formUpload{}
}

var chooseFilesAgain = view.Notice{
	Role: view.NoticeAlert, Title: "Files were not saved",
	Body: "Choose the files again before saving, including the photo. The browser cannot restore file selections after this page changes.",
}

func (u *formUpload) close(ctx context.Context) {
	if !u.saved {
		takeBack(ctx, u.placed)
	}
}

func (s *Server) readFormUpload(r *http.Request, c *collectionCtx, uid string) (*formUpload, error) {
	u := &formUpload{}
	*r = *r.WithContext(context.WithValue(r.Context(), formUploadKey{}, u))
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return u, nil
	}
	up, err := uploadOf(r)
	if err != nil {
		return u, err
	}
	var obj *model.Object
	if c.cal != nil {
		if uid != "" {
			obj, err = c.find(r.Context(), uid)
		} else {
			obj, err = cal.NewObject(c.spec.component, "pending-upload", time.Now())
		}
		if err != nil {
			n := s.unavailableNotice(c, c.listPath(), err)
			u.notice = &n
		}
	}
	remaining := attachLimit(c.col)
	for {
		part, err := nextFile(up)
		if err != nil {
			return u, err
		}
		if part == nil {
			break
		}
		u.chosen = true
		up.remain -= int64(len(part.FileName()) + len(part.Header.Get("Content-Type")))
		if up.remain < 0 {
			return u, &http.MaxBytesError{Limit: conf.MaxFormBodyBytes}
		}
		if u.notice != nil {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return u, err
			}
			continue
		}
		if c.card != nil && part.FormName() == "photo" && u.photo == nil {
			src, err := photo.Load(part, s.cfg.PhotoMaxUploadBytes, s.cfg.PhotoMaxPixels)
			if err == nil {
				u.photo, err = photo.Render(src, photo.DefaultCrop, s.cfg.PhotoMaxSide, s.cfg.PhotoJPEGQuality)
			}
			if err != nil {
				n := s.photoRefusal(err)
				u.notice = &n
			}
			continue
		}
		if c.cal == nil || part.FormName() != "file" {
			u.notice = &view.Notice{Role: view.NoticeAlert, Title: "Nothing was saved", Body: "The form contains an unexpected file field. Open the form again."}
			continue
		}
		head, err := io.ReadAll(io.LimitReader(part, remaining+1))
		if err != nil {
			return u, err
		}
		f := attachFile{name: uploadName(part.FileName()), mediaType: http.DetectContentType(head), head: head, size: int64(len(head)), rest: bytes.NewReader(nil)}
		if int64(len(head)) > remaining {
			p, n := s.placeAttachment(r.Context(), c, obj, f.name, f.mediaType, io.MultiReader(bytes.NewReader(head), part), attachLimit(c.col), declaredSizeValues(up.fields, f.name))
			if n != nil {
				u.notice = n
				continue
			}
			u.placed = append(u.placed, p)
			f.head, f.link, f.size = nil, p.uri, p.size
		} else {
			remaining -= int64(len(head))
		}
		u.files = append(u.files, f)
	}
	r.PostForm = up.fields
	r.Form = maps.Clone(up.fields)
	for k, vs := range r.URL.Query() {
		if _, ok := r.Form[k]; !ok {
			r.Form[k] = vs
		}
	}
	return u, nil
}

// apply adds uploads after the ordinary patch has passed validation. The
// final record's size decides whether a small file still fits inside it.
func (s *Server) applyFormUpload(r *http.Request, c *collectionCtx, obj *model.Object, u *formUpload) (*model.Object, *view.Notice) {
	if u.photo != nil {
		if photoOf(obj) == linkedPhoto {
			return nil, &view.Notice{Role: view.NoticeAlert, Title: "Nothing was saved", Body: externalPhotoNote}
		}
		p, err := model.NewPatch(card.PhotoOps(obj, u.photo)...)
		if err == nil {
			err = p.Apply(obj)
		}
		if err != nil {
			n := s.writeFailure(c, "PUT", obj.Path, err)
			return nil, &n
		}
	}
	for _, f := range u.files {
		// Include all earlier patches when deciding the finished size.
		settled, err := settled(obj)
		if err != nil {
			n := s.writeFailure(c, "PUT", obj.Path, err)
			return nil, &n
		}
		next, placed, n := s.attached(r.Context(), c, settled, f)
		if n != nil {
			return nil, n
		}
		if placed != nil {
			u.placed = append(u.placed, placed)
		}
		obj = next
	}
	return obj, nil
}
