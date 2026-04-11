package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/crazy-max/diun/v4/internal/model"
	"github.com/crazy-max/diun/v4/pkg/registry"
	"github.com/rs/zerolog/log"
)

// ImageResponse is the JSON payload returned by /api/images
type ImageResponse struct {
	Images []ImageEntry `json:"images"`
}

// ImageEntry represents a single image row in the portal
type ImageEntry struct {
	Name           string            `json:"name"`
	Tag            string            `json:"tag"`
	Status         model.ImageStatus `json:"status"`
	Provider       string            `json:"provider"`
	ContainerID    string            `json:"container_id,omitempty"`
	ContainerNames string            `json:"container_names,omitempty"`
	Digest         string            `json:"digest"`
	Platform       string            `json:"platform"`
	UpdatedAt      string            `json:"updated_at"`
}

// handleAPIImages returns the list of watched images with their last known status as JSON
func (s *Server) handleAPIImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Collect entries from in-memory status map (populated after each scan)
	s.mu.RLock()
	entries := make([]ImageEntry, 0, len(s.statuses))
	for _, se := range s.statuses {
		entries = append(entries, statusEntryToImageEntry(se))
	}
	s.mu.RUnlock()

	// Also include any images from the DB that have no in-memory status yet
	dbImages, err := s.db.ListImage()
	if err != nil {
		log.Error().Err(err).Msg("Cannot list images from db")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		seen[e.Name+":"+e.Tag] = true
	}
	for name, manifests := range dbImages {
		for _, mfst := range manifests {
			key := name + ":" + mfst.Tag
			if !seen[key] {
				entries = append(entries, manifestToImageEntry(mfst))
			}
		}
	}

	// Sort: updates first, then new, then unchanged, then others; within same status by name
	sort.Slice(entries, func(i, j int) bool {
		pi := statusPriority(entries[i].Status)
		pj := statusPriority(entries[j].Status)
		if pi != pj {
			return pi < pj
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Tag < entries[j].Tag
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ImageResponse{Images: entries}); err != nil {
		log.Error().Err(err).Msg("Cannot encode image response")
	}
}

// handleAPIContainers handles container actions; currently only "rebuild"
// Path pattern: /api/containers/{containerID}/rebuild
func (s *Server) handleAPIContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract container ID and action from path
	// e.g. /api/containers/abc123/rebuild
	path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "rebuild" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	containerID := parts[0]
	if containerID == "" {
		http.Error(w, "container ID required", http.StatusBadRequest)
		return
	}

	cli, err := s.newDockerClient()
	if err != nil {
		log.Error().Err(err).Msg("Cannot create Docker client for rebuild")
		http.Error(w, "cannot connect to Docker: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer cli.Close()

	newID, err := cli.RebuildContainer(containerID)
	if err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Container rebuild failed")
		http.Error(w, "rebuild failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":       "ok",
		"new_container": newID,
	})
}

// handleUI serves the single-page HTML portal
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(portalHTML))
}

func statusEntryToImageEntry(se StatusEntry) ImageEntry {
	return ImageEntry{
		Name:           se.Image.Name(),
		Tag:            se.Image.Tag,
		Status:         se.Status,
		Provider:       se.Provider,
		ContainerID:    se.ContainerID,
		ContainerNames: se.ContainerNames,
		Digest:         shortDigest(se.Manifest.Digest.String()),
		Platform:       se.Manifest.Platform,
		UpdatedAt:      se.UpdatedAt.UTC().Format("2006-01-02 15:04:05"),
	}
}

func manifestToImageEntry(m registry.Manifest) ImageEntry {
	updatedAt := ""
	if m.Created != nil {
		updatedAt = m.Created.UTC().Format("2006-01-02 15:04:05")
	}
	return ImageEntry{
		Name:      m.Name,
		Tag:       m.Tag,
		Status:    model.ImageStatusUnchange,
		Digest:    shortDigest(m.Digest.String()),
		Platform:  m.Platform,
		UpdatedAt: updatedAt,
	}
}

func shortDigest(d string) string {
	const prefix = "sha256:"
	if strings.HasPrefix(d, prefix) && len(d) > len(prefix)+12 {
		return d[:len(prefix)+12]
	}
	return d
}

func statusPriority(s model.ImageStatus) int {
	switch s {
	case model.ImageStatusUpdate:
		return 0
	case model.ImageStatusNew:
		return 1
	case model.ImageStatusUnchange:
		return 2
	case model.ImageStatusSkip:
		return 3
	case model.ImageStatusError:
		return 4
	default:
		return 5
	}
}

// portalHTML is the single-page web portal UI
const portalHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Diun — Image Update Portal</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
    header { background: #1e293b; padding: 1rem 2rem; display: flex; align-items: center; gap: 1rem; border-bottom: 1px solid #334155; }
    header h1 { font-size: 1.4rem; font-weight: 700; }
    header .badge { font-size: .75rem; background: #3b82f6; color: #fff; padding: .2rem .5rem; border-radius: 9999px; }
    .toolbar { padding: 1rem 2rem; display: flex; gap: .75rem; flex-wrap: wrap; align-items: center; }
    .toolbar label { font-size: .875rem; color: #94a3b8; }
    select, button { border: none; border-radius: .375rem; padding: .4rem .8rem; cursor: pointer; font-size: .875rem; }
    select { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; }
    button.refresh { background: #3b82f6; color: #fff; }
    button.refresh:hover { background: #2563eb; }
    button.rebuild { background: #f59e0b; color: #1a1a1a; font-weight: 600; }
    button.rebuild:hover { background: #d97706; }
    button.rebuild:disabled { background: #6b7280; cursor: not-allowed; }
    .count-bar { padding: 0 2rem .5rem; font-size: .8rem; color: #94a3b8; }
    table { width: 100%; border-collapse: collapse; }
    thead th { position: sticky; top: 0; background: #1e293b; padding: .6rem 1rem; text-align: left; font-size: .75rem; font-weight: 600; text-transform: uppercase; letter-spacing: .05em; color: #94a3b8; cursor: pointer; user-select: none; }
    thead th:hover { color: #e2e8f0; }
    thead th .sort-icon { margin-left: .25rem; }
    tbody tr { border-bottom: 1px solid #1e293b; transition: background .1s; }
    tbody tr:hover { background: #1e293b; }
    td { padding: .6rem 1rem; font-size: .875rem; white-space: nowrap; }
    .status-badge { display: inline-flex; align-items: center; gap: .3rem; padding: .2rem .6rem; border-radius: 9999px; font-size: .75rem; font-weight: 600; }
    .status-update { background: #fef08a22; color: #fbbf24; }
    .status-new    { background: #a7f3d022; color: #34d399; }
    .status-unchange { background: #e2e8f011; color: #94a3b8; }
    .status-skip   { background: #e2e8f011; color: #64748b; }
    .status-error  { background: #fecaca22; color: #f87171; }
    .image-name { font-weight: 600; color: #93c5fd; }
    .tag { color: #c4b5fd; }
    .digest { font-family: monospace; color: #64748b; font-size: .8rem; }
    .empty { text-align: center; padding: 4rem; color: #64748b; }
    .spinner { display: inline-block; width: 1rem; height: 1rem; border: 2px solid #334155; border-top-color: #3b82f6; border-radius: 50%; animation: spin .6s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .toast { position: fixed; bottom: 1.5rem; right: 1.5rem; padding: .75rem 1.25rem; border-radius: .5rem; font-size: .875rem; z-index: 999; animation: fadein .3s ease; }
    .toast-ok  { background: #166534; color: #bbf7d0; }
    .toast-err { background: #7f1d1d; color: #fecaca; }
    @keyframes fadein { from { opacity: 0; transform: translateY(.5rem); } to { opacity: 1; transform: none; } }
    .table-wrap { overflow-x: auto; padding: 0 2rem 2rem; }
  </style>
</head>
<body>
<header>
  <h1>🐋 Diun</h1>
  <span class="badge">Image Update Portal</span>
</header>
<div class="toolbar">
  <label for="filter-status">Status:</label>
  <select id="filter-status">
    <option value="">All</option>
    <option value="update">Update available</option>
    <option value="new">New</option>
    <option value="unchange">Unchanged</option>
    <option value="skip">Skipped</option>
    <option value="error">Error</option>
  </select>
  <label for="filter-search">Search:</label>
  <input id="filter-search" type="text" placeholder="image name…" style="background:#1e293b;border:1px solid #334155;color:#e2e8f0;border-radius:.375rem;padding:.4rem .8rem;font-size:.875rem;width:14rem;">
  <button class="refresh" id="btn-refresh">↻ Refresh</button>
</div>
<div class="count-bar" id="count-bar"></div>
<div class="table-wrap">
<table>
  <thead>
    <tr>
      <th data-col="status">Status <span class="sort-icon"></span></th>
      <th data-col="name">Image <span class="sort-icon"></span></th>
      <th data-col="tag">Tag <span class="sort-icon"></span></th>
      <th data-col="platform">Platform <span class="sort-icon"></span></th>
      <th data-col="digest">Digest <span class="sort-icon"></span></th>
      <th data-col="updated_at">Last seen <span class="sort-icon"></span></th>
      <th data-col="provider">Provider <span class="sort-icon"></span></th>
      <th>Actions</th>
    </tr>
  </thead>
  <tbody id="tbody"></tbody>
</table>
</div>
<script>
const STATUS_ORDER = {update:0, new:1, unchange:2, skip:3, error:4};
let allImages = [];
let sortCol = 'status', sortAsc = true;

function statusBadge(s) {
  const icons = {update:'⬆', new:'✦', unchange:'✔', skip:'—', error:'✖'};
  return ` + "`" + `<span class="status-badge status-${s}">${icons[s]||'?'} ${s}</span>` + "`" + `;
}

function render() {
  const filterStatus = document.getElementById('filter-status').value;
  const filterSearch = document.getElementById('filter-search').value.toLowerCase();
  let rows = allImages.filter(img => {
    if (filterStatus && img.status !== filterStatus) return false;
    if (filterSearch && !img.name.toLowerCase().includes(filterSearch) && !img.tag.toLowerCase().includes(filterSearch)) return false;
    return true;
  });

  rows.sort((a, b) => {
    let av = sortCol === 'status' ? (STATUS_ORDER[a.status]??99) : (a[sortCol]||'');
    let bv = sortCol === 'status' ? (STATUS_ORDER[b.status]??99) : (b[sortCol]||'');
    if (av < bv) return sortAsc ? -1 : 1;
    if (av > bv) return sortAsc ? 1 : -1;
    return 0;
  });

  document.getElementById('count-bar').textContent = rows.length + ' image' + (rows.length !== 1 ? 's' : '') + ' shown';

  const tbody = document.getElementById('tbody');
  if (rows.length === 0) {
    tbody.innerHTML = '<tr><td colspan="8" class="empty">No images found. Run a scan first.</td></tr>';
    return;
  }
  tbody.innerHTML = rows.map(img => {
    const canRebuild = (img.status === 'update' || img.status === 'new') && img.container_id;
    const rebuildBtn = canRebuild
      ? ` + "`" + `<button class="rebuild" onclick="rebuild('${img.container_id}', this)" title="Pull new image and recreate container ${img.container_names||img.container_id}">⟳ Rebuild</button>` + "`" + `
      : '';
    return ` + "`" + `<tr>
      <td>${statusBadge(img.status)}</td>
      <td class="image-name">${escHtml(img.name)}</td>
      <td class="tag">${escHtml(img.tag)}</td>
      <td>${escHtml(img.platform)}</td>
      <td class="digest">${escHtml(img.digest)}</td>
      <td>${escHtml(img.updated_at)}</td>
      <td>${escHtml(img.provider)}</td>
      <td>${rebuildBtn}</td>
    </tr>` + "`" + `;
  }).join('');

  // Update sort icons
  document.querySelectorAll('thead th[data-col]').forEach(th => {
    const icon = th.querySelector('.sort-icon');
    if (th.dataset.col === sortCol) {
      icon.textContent = sortAsc ? '▲' : '▼';
    } else {
      icon.textContent = '';
    }
  });
}

function escHtml(s) {
  return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

async function loadImages() {
  const btn = document.getElementById('btn-refresh');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  try {
    const resp = await fetch('/api/images');
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    const data = await resp.json();
    allImages = data.images || [];
    render();
  } catch(e) {
    showToast('Failed to load images: ' + e.message, false);
  } finally {
    btn.disabled = false;
    btn.textContent = '↻ Refresh';
  }
}

async function rebuild(containerID, btn) {
  if (!confirm('Rebuild container ' + containerID + '?\n\nThis will pull the latest image, stop, remove and recreate the container.')) return;
  btn.disabled = true;
  btn.textContent = '…';
  try {
    const resp = await fetch('/api/containers/' + containerID + '/rebuild', {method:'POST'});
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.message || 'HTTP ' + resp.status);
    showToast('Container rebuilt successfully (' + (data.new_container||'') + ')', true);
    setTimeout(loadImages, 1000);
  } catch(e) {
    showToast('Rebuild failed: ' + e.message, false);
    btn.disabled = false;
    btn.textContent = '⟳ Rebuild';
  }
}

function showToast(msg, ok) {
  const t = document.createElement('div');
  t.className = 'toast ' + (ok ? 'toast-ok' : 'toast-err');
  t.textContent = msg;
  document.body.appendChild(t);
  setTimeout(() => t.remove(), 4000);
}

document.getElementById('btn-refresh').addEventListener('click', loadImages);
document.getElementById('filter-status').addEventListener('change', render);
document.getElementById('filter-search').addEventListener('input', render);
document.querySelectorAll('thead th[data-col]').forEach(th => {
  th.addEventListener('click', () => {
    const col = th.dataset.col;
    if (sortCol === col) sortAsc = !sortAsc;
    else { sortCol = col; sortAsc = true; }
    render();
  });
});

loadImages();
</script>
</body>
</html>`
