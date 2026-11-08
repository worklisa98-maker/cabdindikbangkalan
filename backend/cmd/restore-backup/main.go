package main

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var magic = []byte("SIPRESBK1")

func key(raw string) ([]byte, error) {
	if b, e := base64.StdEncoding.DecodeString(raw); e == nil && len(b) == 32 {
		return b, nil
	}
	if b, e := hex.DecodeString(raw); e == nil && len(b) == 32 {
		return b, nil
	}
	return nil, errors.New("BACKUP_ENCRYPTION_KEY tidak valid")
}
func decrypt(src, dst string, k []byte) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	head := make([]byte, len(magic))
	if _, e = io.ReadFull(in, head); e != nil || string(head) != string(magic) {
		return errors.New("format backup tidak valid")
	}
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer out.Close()
	b, _ := aes.NewCipher(k)
	g, _ := cipher.NewGCM(b)
	counter := uint64(0)
	for {
		l := make([]byte, 4)
		_, e = io.ReadFull(in, l)
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		n := uint32(l[0])<<24 | uint32(l[1])<<16 | uint32(l[2])<<8 | uint32(l[3])
		if n > 2<<20 {
			return errors.New("chunk rusak")
		}
		nonce := make([]byte, g.NonceSize())
		if _, e = io.ReadFull(in, nonce); e != nil {
			return e
		}
		sealed := make([]byte, n)
		if _, e = io.ReadFull(in, sealed); e != nil {
			return e
		}
		counter++
		plain, e := g.Open(nil, nonce, sealed, []byte(strconv.FormatUint(counter, 10)))
		if e != nil {
			return errors.New("key salah atau backup rusak")
		}
		if _, e = out.Write(plain); e != nil {
			return e
		}
	}
}
func extract(src, dst string, dry bool) error {
	z, e := zip.OpenReader(src)
	if e != nil {
		return e
	}
	defer z.Close()
	root, _ := filepath.Abs(dst)
	for _, f := range z.File {
		target, _ := filepath.Abs(filepath.Join(root, filepath.FromSlash(f.Name)))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return errors.New("path traversal ditolak")
		}
		fmt.Println(f.Name)
		if dry || f.FileInfo().IsDir() {
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		r, e := f.Open()
		if e != nil {
			return e
		}
		w, e := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e == nil {
			_, e = io.Copy(w, r)
		}
		r.Close()
		w.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
func main() {
	dry := flag.Bool("dry-run", false, "verifikasi tanpa memulihkan")
	force := flag.Bool("force", false, "izinkan target storage tidak kosong")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: restore-backup [--dry-run] [--force] FILE")
		os.Exit(2)
	}
	k, e := key(os.Getenv("BACKUP_ENCRYPTION_KEY"))
	if e != nil {
		panic(e)
	}
	tmp, e := os.MkdirTemp("", "sipres-restore-*")
	if e != nil {
		panic(e)
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, "payload.zip")
	if e = decrypt(flag.Arg(0), archive, k); e != nil {
		panic(e)
	}
	storage := os.Getenv("STORAGE_LOCAL_DIR")
	db := os.Getenv("DATABASE_URL")
	if storage == "" {
		storage = "./uploads"
	}
	entries, _ := os.ReadDir(storage)
	if !*dry && len(entries) > 0 && !*force {
		panic("target storage tidak kosong; gunakan --force setelah memastikan backup target")
	}
	out := filepath.Join(tmp, "extract")
	if e = extract(archive, out, *dry); e != nil {
		panic(e)
	}
	if *dry {
		fmt.Println("Dry-run berhasil: format dan autentikasi enkripsi valid")
		return
	}
	if db == "" {
		panic("DATABASE_URL wajib")
	}
	dump := filepath.Join(out, "database", "database.dump")
	cmd := exec.Command("pg_restore", "--clean", "--if-exists", "--no-owner", "--dbname", db, dump)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		panic(e)
	}
	src := filepath.Join(out, "storage")
	_ = filepath.Walk(src, func(p string, i os.FileInfo, e error) error {
		if e != nil || i.IsDir() {
			return e
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(storage, rel)
		_ = os.MkdirAll(filepath.Dir(dst), 0700)
		r, _ := os.Open(p)
		defer r.Close()
		w, _ := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		defer w.Close()
		_, e = io.Copy(w, r)
		return e
	})
	fmt.Println("Pemulihan selesai; jalankan migrasi aplikasi dan pemeriksaan kesehatan.")
}
