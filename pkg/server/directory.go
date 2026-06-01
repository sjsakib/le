package server

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

//go:embed templates/directory.html
var templateFS embed.FS

var dirTemplate = template.Must(template.ParseFS(templateFS, "templates/directory.html"))

type FileInfo struct {
	Name         string
	Path         string
	Size         string
	SizeBytes    int64
	Modified     string
	ModifiedTime time.Time
	IsDir        bool
	IsCode       bool
	IsImage      bool
	IsAudio      bool
	IsVideo      bool
	IsArchive    bool
	IsText       bool
}

type Breadcrumb struct {
	Name   string
	Path   string
	IsLast bool
}

type DirectoryData struct {
	Path        string
	ParentPath  string
	Files       []FileInfo
	Breadcrumbs []Breadcrumb
	SearchQuery string
	Sort        string
	Order       string
	NameSortURL string
	DateSortURL string
	SizeSortURL string
}

const (
	directorySortName = "name"
	directorySortDate = "date"
	directorySortSize = "size"
	directorySortAsc  = "asc"
	directorySortDesc = "desc"
)

func isCodeFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	codeExts := []string{
		".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".java", ".c", ".cpp",
		".h", ".hpp", ".cs", ".php", ".rb", ".swift", ".kt", ".rs", ".scala",
		".html", ".css", ".scss", ".sass", ".vue", ".json", ".xml", ".yaml",
		".yml", ".toml", ".sql", ".sh", ".bash", ".zsh", ".fish", ".ps1",
	}
	return slices.Contains(codeExts, ext)
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico"}
	return slices.Contains(imageExts, ext)
}

func isAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	audioExts := []string{".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac", ".wma"}
	return slices.Contains(audioExts, ext)
}

func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg"}
	return slices.Contains(videoExts, ext)
}

func isArchiveFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	archiveExts := []string{".zip", ".tar", ".gz", ".bz2", ".xz", ".rar", ".7z", ".tar.gz", ".tar.bz2"}
	for _, e := range archiveExts {
		if ext == e || strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	textExts := []string{".txt", ".md", ".markdown", ".rst", ".log", ".csv", ".tsv"}
	return slices.Contains(textExts, ext)
}

func humanizeSize(size int64) string {
	if size == 0 {
		return ""
	}

	const unit = 1024.0
	if size < unit {
		return convertIntToString(int(size)) + " B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	exp := 0
	val := float64(size)

	for val >= unit && exp < len(units)-1 {
		val /= unit
		exp++
	}

	if val < 10 {
		// cause sprintf would convert 1.0 to 1
		whole := int(val)
		frac := int((val - float64(whole)) * 10)
		if frac == 0 {
			return convertIntToString(whole) + " " + units[exp]
		}
		return convertIntToString(whole) + "." + convertIntToString(frac) + " " + units[exp]
	}

	return convertIntToString(int(val)) + " " + units[exp]
}

func normalizeDirectorySort(sortBy string) string {
	if sortBy == directorySortName {
		return directorySortName
	}
	if sortBy == directorySortSize {
		return directorySortSize
	}
	return directorySortDate
}

func normalizeDirectoryOrder(order string) string {
	if order == directorySortAsc {
		return directorySortAsc
	}
	return directorySortDesc
}

func defaultDirectoryOrder(sortBy string) string {
	if sortBy == directorySortName {
		return directorySortAsc
	}
	return directorySortDesc
}

func nextDirectorySortOrder(currentSort, currentOrder, sortBy string) string {
	if currentSort == sortBy {
		if currentOrder == directorySortAsc {
			return directorySortDesc
		}
		return directorySortAsc
	}
	return defaultDirectoryOrder(sortBy)
}

func directorySortURL(searchQuery, currentSort, currentOrder, sortBy string) string {
	values := url.Values{}
	if searchQuery != "" {
		values.Set("q", searchQuery)
	}
	values.Set("sort", sortBy)
	values.Set("order", nextDirectorySortOrder(currentSort, currentOrder, sortBy))
	return "?" + values.Encode()
}

func compareFileName(left, right string, ascending bool) bool {
	left = strings.ToLower(left)
	right = strings.ToLower(right)
	if ascending {
		return left < right
	}
	return left > right
}

func convertIntToString(n int) string {
	if n == 0 {
		return "0"
	}

	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return convertIntToString(mins) + " minutes ago"
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return convertIntToString(hours) + " hours ago"
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return convertIntToString(days) + " days ago"
	}

	return t.Format("Jan 2, 2006")
}

func (h *handler) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	sortBy := normalizeDirectorySort(r.URL.Query().Get("sort"))
	sortOrder := defaultDirectoryOrder(sortBy)
	if r.URL.Query().Has("order") {
		sortOrder = normalizeDirectoryOrder(r.URL.Query().Get("order"))
	}
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	normalizedSearchQuery := strings.ToLower(searchQuery)
	var dirs, regularFiles []FileInfo

	for _, file := range files {

		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		if normalizedSearchQuery != "" && !strings.Contains(strings.ToLower(file.Name()), normalizedSearchQuery) {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		urlPath := filepath.Join(r.URL.Path, file.Name())
		if !strings.HasPrefix(urlPath, "/") {
			urlPath = "/" + urlPath
		}

		fileInfo := FileInfo{
			Name:         file.Name(),
			Path:         urlPath,
			Modified:     formatTime(info.ModTime()),
			ModifiedTime: info.ModTime(),
			IsDir:        file.IsDir(),
		}

		if file.IsDir() {
			fileInfo.Name += "/"
			dirs = append(dirs, fileInfo)
		} else {
			fileInfo.Size = humanizeSize(info.Size())
			fileInfo.SizeBytes = info.Size()
			fileInfo.IsCode = isCodeFile(file.Name())
			fileInfo.IsImage = isImageFile(file.Name())
			fileInfo.IsAudio = isAudioFile(file.Name())
			fileInfo.IsVideo = isVideoFile(file.Name())
			fileInfo.IsArchive = isArchiveFile(file.Name())
			fileInfo.IsText = isTextFile(file.Name())
			regularFiles = append(regularFiles, fileInfo)
		}
	}

	// file and directory sorting
	sort.Slice(dirs, func(i, j int) bool {
		if sortBy == directorySortDate && !dirs[i].ModifiedTime.Equal(dirs[j].ModifiedTime) {
			if sortOrder == directorySortAsc {
				return dirs[i].ModifiedTime.Before(dirs[j].ModifiedTime)
			}
			return dirs[i].ModifiedTime.After(dirs[j].ModifiedTime)
		}
		return compareFileName(dirs[i].Name, dirs[j].Name, sortBy != directorySortName || sortOrder == directorySortAsc)
	})
	sort.Slice(regularFiles, func(i, j int) bool {
		if sortBy == directorySortSize && regularFiles[i].SizeBytes != regularFiles[j].SizeBytes {
			if sortOrder == directorySortAsc {
				return regularFiles[i].SizeBytes < regularFiles[j].SizeBytes
			}
			return regularFiles[i].SizeBytes > regularFiles[j].SizeBytes
		}
		if sortBy == directorySortDate && !regularFiles[i].ModifiedTime.Equal(regularFiles[j].ModifiedTime) {
			if sortOrder == directorySortAsc {
				return regularFiles[i].ModifiedTime.Before(regularFiles[j].ModifiedTime)
			}
			return regularFiles[i].ModifiedTime.After(regularFiles[j].ModifiedTime)
		}
		return compareFileName(regularFiles[i].Name, regularFiles[j].Name, sortBy != directorySortName || sortOrder == directorySortAsc)
	})

	// directory first and then regular files
	allFiles := append(dirs, regularFiles...)

	var breadcrumbs []Breadcrumb
	if r.URL.Path != "/" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		breadcrumbs = append(breadcrumbs, Breadcrumb{Name: "Root", Path: "/"})

		currentPath := ""
		for i, part := range parts {
			currentPath += "/" + part
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Name:   part,
				Path:   currentPath,
				IsLast: i == len(parts)-1,
			})
		}
	} else {
		breadcrumbs = append(breadcrumbs, Breadcrumb{Name: "Root", Path: "/", IsLast: true})
	}

	parentPath := ""
	if r.URL.Path != "/" {
		parentPath = filepath.Dir(r.URL.Path)
		if parentPath == "." {
			parentPath = "/"
		}
	}

	data := DirectoryData{
		Path:        r.URL.Path,
		ParentPath:  parentPath,
		Files:       allFiles,
		Breadcrumbs: breadcrumbs,
		SearchQuery: searchQuery,
		Sort:        sortBy,
		Order:       sortOrder,
		NameSortURL: directorySortURL(searchQuery, sortBy, sortOrder, directorySortName),
		DateSortURL: directorySortURL(searchQuery, sortBy, sortOrder, directorySortDate),
		SizeSortURL: directorySortURL(searchQuery, sortBy, sortOrder, directorySortSize),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dirTemplate.Execute(w, data); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}
