package gosync

import (
	"os"
	"path/filepath"
)

// walk源host要同步的文件, 生成md5, 并返回列表
func Traverse(path string) ([]string, error) {
	f, fErr := os.Lstat(path)
	if fErr != nil {
		return nil, fErr
	}
	var dir string
	var base string
	if f.IsDir() {
		dir = path
		base = "."
	} else {
		dir = filepath.Dir(path)
		base = filepath.Base(path)
	}
	fErr = os.Chdir(dir)
	if fErr != nil {
		return nil, fErr
	}
	md5List := make([]string, 10)
	var md5Str string
	WalkFunc := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			md5Str = path + ",,Directory"
			md5List = append(md5List, md5Str)
		}
		if info.Mode().IsRegular() {
			// if !info.IsDir() {
			md5Str, fErr = Md5OfAFile(path)
			if fErr != nil {
				PrintInfor(fErr)
				return fErr
			}
			md5Str = path + ",," + md5Str
			md5List = append(md5List, md5Str)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			filename, err := os.Readlink(path)
			if err != nil {
				return err
			}
			md5Str = "symbolLink&&" + filename
			md5Str = path + ",," + md5Str
			md5List = append(md5List, md5Str)
		}
		return nil
	}
	fErr = filepath.Walk(base, WalkFunc)
	if fErr != nil {
		PrintInfor(fErr)
		return nil, fErr
	}
	// DubugInfor(md5List)
	for i, v := range md5List {
		// DubugInfor(v)
		if v == "" || v == ".,,Directory" {
			continue
		}
		md5List = md5List[i:]
		break
	}
	if md5List[len(md5List)-1] == ".,,Directory" {
		return []string{}, nil
	}
	return md5List, nil
}
