<script lang="ts">
  import { onMount } from 'svelte';

  // ── Types ──────────────────────────────────────────────────────────────────
  interface ImageFile {
    src: string;
    thumb: string;
    name: string;
    date: Date | null;
    resolution: string | null;
    id: string;
  }
  interface EditState {
    rotation: number; // 0 | 90 | 180 | 270
    flipH: boolean;
    flipV: boolean;
    brightness: number; // 0–200 (100 = normal)
    contrast: number;
    saturation: number;
    hue: number; // 0–360
    vignette: number; // 0–100
    crop: { x: number; y: number; w: number; h: number } | null;
  }
  const DEFAULT_EDIT: EditState = {
    rotation: 0,
    flipH: false,
    flipV: false,
    brightness: 100,
    contrast: 100,
    saturation: 100,
    hue: 0,
    vignette: 0,
    crop: null,
  };

  // ── localStorage persistence ───────────────────────────────────────────────
  const LS_KEY = 'vrsync-edits-v2';
  function loadEdits(): Record<string, EditState> {
    try {
      return JSON.parse(localStorage.getItem(LS_KEY) ?? '{}');
    } catch {
      return {};
    }
  }
  function saveEdits(edits: Record<string, EditState>) {
    localStorage.setItem(LS_KEY, JSON.stringify(edits));
  }
  let allEdits = $state<Record<string, EditState>>({});
  function getEdit(src: string): EditState {
    return allEdits[src] ?? { ...DEFAULT_EDIT };
  }
  function setEdit(src: string, e: EditState) {
    allEdits = { ...allEdits, [src]: e };
    saveEdits(allEdits);
  }
  function clearEdit(src: string) {
    const copy = { ...allEdits };
    delete copy[src];
    allEdits = copy;
    saveEdits(allEdits);
  }
  function isEdited(e: EditState): boolean {
    return JSON.stringify(e) !== JSON.stringify(DEFAULT_EDIT);
  }

  // ── Gallery state ──────────────────────────────────────────────────────────
  let query = $state('');
  let images = $state<ImageFile[]>([]);
  let loadStates = $state<Record<string, 'loading' | 'loaded' | 'error'>>({});
  let syncStatus = $state<'idle' | 'syncing' | 'done' | 'error'>('idle');
  let charCount = $derived(query.length);
  let page = $state(1);
  let hasMore = $state(true);
  let loadingMore = $state(false);
  let sentinel: HTMLDivElement;
  let deleteTarget = $state<ImageFile | null>(null);
  let deleting = $state(false);

  // ── Editor state ───────────────────────────────────────────────────────────
  let editorOpen = $state(false);
  let editorImage = $state<ImageFile | null>(null);
  let edit = $state<EditState>({ ...DEFAULT_EDIT });
  let sidebarOpen = $state(true); // collapsible panel

  // Crop
  let cropMode = $state(false);
  let cropStart = $state<{ x: number; y: number } | null>(null);
  let cropDraft = $state<{ x: number; y: number; w: number; h: number } | null>(
    null,
  );

  // Auto-save edits whenever `edit` changes while editor is open
  let autoSaveTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    // Access edit fields so the effect tracks them
    const snapshot = { ...edit };
    if (!editorOpen || !editorImage) return;
    clearTimeout(autoSaveTimer);
    autoSaveTimer = setTimeout(() => {
      setEdit(editorImage!.src, snapshot);
    }, 600);
  });

  // ── Helpers ────────────────────────────────────────────────────────────────
  function parseFilename(
    path: string,
  ): Pick<ImageFile, 'date' | 'resolution' | 'name'> {
    const name = path.split('/').pop() ?? path;
    const dm = name.match(/(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})/);
    const rm = name.match(/(\d{3,5}x\d{3,5})/);
    return {
      name,
      date: dm
        ? new Date(`${dm[1]}-${dm[2]}-${dm[3]}T${dm[4]}:${dm[5]}:${dm[6]}`)
        : null,
      resolution: rm ? rm[1] : null,
    };
  }
  function formatDate(d: Date): string {
    return d.toLocaleString('en-GB', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
  function toThumb(src: string): string {
    const parts = src.split('/');
    parts.splice(parts.length - 1, 0, 'thumbnails');
    return parts.join('/');
  }
  function normalise(raw: string | { src: string; id?: string }) {
    if (typeof raw === 'string') return { src: raw, id: btoa(raw) };
    return { src: raw.src, id: raw.id ?? btoa(raw.src) };
  }
  function editToFilter(e: EditState): string {
    return [
      `brightness(${e.brightness}%)`,
      `contrast(${e.contrast}%)`,
      `saturate(${e.saturation}%)`,
      `hue-rotate(${e.hue}deg)`,
    ].join(' ');
  }
  function editToTransform(e: EditState): string {
    const parts = [`rotate(${e.rotation}deg)`];
    if (e.flipH) parts.push('scaleX(-1)');
    if (e.flipV) parts.push('scaleY(-1)');
    return parts.join(' ');
  }

  // ── Data fetching ──────────────────────────────────────────────────────────
  async function loadMore() {
    if (loadingMore || !hasMore) return;
    loadingMore = true;
    try {
      const res = await fetch(`/images?page=${page}`);
      if (!res.ok) throw new Error();
      const json = await res.json();
      const raw: (string | { src: string; id?: string })[] = json.files ?? [];
      const total: number = json.total ?? Infinity;
      const next: ImageFile[] = raw.map((r) => {
        const { src, id } = normalise(r);
        return { src, thumb: toThumb(src), id, ...parseFilename(src) };
      });
      for (const img of next) loadStates[img.src] = 'loading';
      images = [...images, ...next];
      hasMore = images.length < total && next.length > 0;
      page++;
    } catch {
      hasMore = false;
    } finally {
      loadingMore = false;
    }
  }

  onMount(async () => {
    allEdits = loadEdits();
    await loadMore();
    const obs = new IntersectionObserver(
      (e) => {
        if (e[0].isIntersecting) loadMore();
      },
      { rootMargin: '200px' },
    );
    if (sentinel) obs.observe(sentinel);
    return () => obs.disconnect();
  });

  // ── Auto-sync query ────────────────────────────────────────────────────────
  let debounceTimer: ReturnType<typeof setTimeout>;
  function scheduleSync(value: string) {
    syncStatus = 'syncing';
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      fetch('/copy', {
        method: 'POST',
        body: JSON.stringify({ query: value }),
        headers: { 'content-type': 'application/json' },
      })
        .then((r) => {
          syncStatus = r.ok ? 'done' : 'error';
          setTimeout(() => (syncStatus = 'idle'), 2000);
        })
        .catch(() => {
          syncStatus = 'error';
          setTimeout(() => (syncStatus = 'idle'), 2000);
        });
    }, 500);
  }
  function handleInput(e: Event) {
    const v = (e.currentTarget as HTMLTextAreaElement).value;
    query = v;
    scheduleSync(v);
  }
  function clearQuery() {
    query = '';
    scheduleSync('');
  }

  // ── Load state ─────────────────────────────────────────────────────────────
  function onLoad(src: string) {
    loadStates[src] = 'loaded';
  }
  function onError(src: string) {
    loadStates[src] = 'error';
  }

  // ── Editor open/close ──────────────────────────────────────────────────────
  function openEditor(img: ImageFile) {
    editorImage = img;
    edit = { ...getEdit(img.src) };
    cropMode = false;
    cropDraft = null;
    cropStart = null;
    editorOpen = true;
    document.body.style.overflow = 'hidden';
  }
  function closeEditor() {
    editorOpen = false;
    editorImage = null;
    cropMode = false;
    cropDraft = null;
    document.body.style.overflow = '';
  }

  // ── Transform ──────────────────────────────────────────────────────────────
  function rotate(deg: number) {
    edit = { ...edit, rotation: (((edit.rotation + deg) % 360) + 360) % 360 };
  }
  function toggleFlipH() {
    edit = { ...edit, flipH: !edit.flipH };
  }
  function toggleFlipV() {
    edit = { ...edit, flipV: !edit.flipV };
  }

  // ── Crop ───────────────────────────────────────────────────────────────────
  function enterCropMode() {
    cropMode = true;
    cropDraft = null;
    cropStart = null;
  }
  function cancelCrop() {
    cropMode = false;
    cropDraft = null;
    cropStart = null;
  }
  function applyCrop() {
    if (cropDraft && cropDraft.w > 0.5 && cropDraft.h > 0.5)
      edit = { ...edit, crop: { ...cropDraft } };
    cropMode = false;
    cropDraft = null;
  }
  function resetCrop() {
    edit = { ...edit, crop: null };
  }

  function pct(el: HTMLElement, cx: number, cy: number) {
    const r = el.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(100, ((cx - r.left) / r.width) * 100)),
      y: Math.max(0, Math.min(100, ((cy - r.top) / r.height) * 100)),
    };
  }
  function onCropDown(e: PointerEvent) {
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    const p = pct(e.currentTarget as HTMLElement, e.clientX, e.clientY);
    cropStart = p;
    cropDraft = { x: p.x, y: p.y, w: 0, h: 0 };
  }
  function onCropMove(e: PointerEvent) {
    if (!cropStart) return;
    const p = pct(e.currentTarget as HTMLElement, e.clientX, e.clientY);
    cropDraft = {
      x: Math.min(cropStart.x, p.x),
      y: Math.min(cropStart.y, p.y),
      w: Math.abs(p.x - cropStart.x),
      h: Math.abs(p.y - cropStart.y),
    };
  }
  function onCropUp() {
    cropStart = null;
  }

  // ── Download with canvas ───────────────────────────────────────────────────
  async function download(
    img: ImageFile,
    e: MouseEvent,
    liveEdit: EditState | undefined = undefined,
  ) {
    e.stopPropagation();
    const ed = liveEdit ?? getEdit(img.src);
    if (!isEdited(ed)) {
      try {
        const blob = await fetch(img.src).then((r) => r.blob());
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = img.name;
        a.click();
        URL.revokeObjectURL(url);
      } catch {
        window.open(img.src, '_blank');
      }
      return;
    }
    const image = new Image();
    image.crossOrigin = 'anonymous';
    image.src = img.src;
    await new Promise((res, rej) => {
      image.onload = res;
      image.onerror = rej;
    });
    const rot = ed.rotation,
      swapped = rot === 90 || rot === 270;
    const srcW = image.naturalWidth,
      srcH = image.naturalHeight;
    let cx = 0,
      cy = 0,
      cw = srcW,
      ch = srcH;
    if (ed.crop) {
      cx = (ed.crop.x / 100) * srcW;
      cy = (ed.crop.y / 100) * srcH;
      cw = (ed.crop.w / 100) * srcW;
      ch = (ed.crop.h / 100) * srcH;
    }
    const outW = swapped ? ch : cw,
      outH = swapped ? cw : ch;
    const canvas = document.createElement('canvas');
    canvas.width = outW;
    canvas.height = outH;
    const ctx = canvas.getContext('2d')!;
    ctx.filter = editToFilter(ed);
    ctx.save();
    ctx.translate(outW / 2, outH / 2);
    ctx.rotate((rot * Math.PI) / 180);
    if (ed.flipH) ctx.scale(-1, 1);
    if (ed.flipV) ctx.scale(1, -1);
    ctx.drawImage(image, cx, cy, cw, ch, -cw / 2, -ch / 2, cw, ch);
    ctx.restore();
    if (ed.vignette > 0) {
      ctx.filter = 'none';
      const g = ctx.createRadialGradient(
        outW / 2,
        outH / 2,
        outW * 0.2,
        outW / 2,
        outH / 2,
        outW * 0.8,
      );
      g.addColorStop(0, 'rgba(0,0,0,0)');
      g.addColorStop(1, `rgba(0,0,0,${ed.vignette / 100})`);
      ctx.fillStyle = g;
      ctx.fillRect(0, 0, outW, outH);
    }
    canvas.toBlob((blob) => {
      if (!blob) return;
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = img.name.replace(/(\.\w+)$/, '_edited$1');
      a.click();
      URL.revokeObjectURL(url);
    }, 'image/png');
  }

  // ── Delete ─────────────────────────────────────────────────────────────────
  async function confirmDelete() {
    if (!deleteTarget) return;
    deleting = true;
    try {
      const res = await fetch(`/delete/${deleteTarget.id}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        images = images.filter((i) => i.src !== deleteTarget!.src);
        if (editorImage?.src === deleteTarget.src) closeEditor();
        deleteTarget = null;
      }
    } finally {
      deleting = false;
    }
  }

  // ── Keyboard shortcuts ─────────────────────────────────────────────────────
  function onKeydown(e: KeyboardEvent) {
    if (!editorOpen) return;
    if (e.key === 'Escape') {
      cropMode ? cancelCrop() : closeEditor();
      return;
    }
    if (e.key === 'r' && !e.ctrlKey && !e.metaKey) rotate(90);
    if (e.key === 'R' && !e.ctrlKey && !e.metaKey) rotate(-90);
    if (e.key === 'h' && !e.ctrlKey && !e.metaKey) toggleFlipH();
    if (e.key === 'v' && !e.ctrlKey && !e.metaKey) toggleFlipV();
    if (e.key === 'c' && !e.ctrlKey && !e.metaKey) enterCropMode();
    if (e.key === 'p' && !e.ctrlKey && !e.metaKey) sidebarOpen = !sidebarOpen;
    if (e.key === 'd' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      download(editorImage!, { stopPropagation: () => {} } as MouseEvent, edit);
    }
  }

  // Derived CSS
  let editorFilter = $derived(editorImage ? editToFilter(edit) : 'none');
  let editorTransform = $derived(editorImage ? editToTransform(edit) : 'none');
  let editorHasEdits = $derived(isEdited(edit));
</script>

<svelte:window onkeydown={onKeydown} />

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="header">
  <div class="header-inner">
    <div class="logo">
      <div class="logo-icon">
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path
            d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96-.46 2.5 2.5 0 0 1-1.064-4.62A3 3 0 0 1 5 14v-2a5 5 0 0 1 5-5V5A2.5 2.5 0 0 1 9.5 2Z"
          />
          <path
            d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96-.46 2.5 2.5 0 0 0 1.064-4.62A3 3 0 0 0 19 14v-2a5 5 0 0 0-5-5V5A2.5 2.5 0 0 0 14.5 2Z"
          />
        </svg>
      </div>
      <span class="logo-text">VR-Sync</span>
    </div>
    <span class="image-count"
      >{images.length} image{images.length !== 1 ? 's' : ''}{hasMore
        ? '+'
        : ''}</span
    >
  </div>
</header>

<main class="main">
  <!-- Query Panel -->
  <section class="query-panel">
    <div class="panel-header">
      <div class="panel-label">
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <polyline points="16 18 22 12 16 6" /><polyline
            points="8 6 2 12 8 18"
          />
        </svg>
        Query
      </div>
      <div class="sync-indicator" data-status={syncStatus}>
        <span class="sync-dot {syncStatus}"></span>
        <span class="sync-label">
          {syncStatus === 'syncing'
            ? 'Syncing…'
            : syncStatus === 'done'
              ? 'Synced'
              : syncStatus === 'error'
                ? 'Error'
                : 'Ready'}
        </span>
      </div>
    </div>
    <textarea
      value={query}
      oninput={handleInput}
      rows={5}
      placeholder="Type your query… auto-synced after 500ms"
      class="query-textarea"
    ></textarea>
    <div class="textarea-footer">
      <span class="char-count">{charCount} chars</span>
      {#if charCount > 0}
        <button class="clear-btn" onclick={clearQuery}>
          <svg
            width="11"
            height="11"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <line x1="18" y1="6" x2="6" y2="18" /><line
              x1="6"
              y1="6"
              x2="18"
              y2="18"
            />
          </svg>
          Clear
        </button>
      {/if}
    </div>
  </section>

  <!-- Gallery -->
  <section class="gallery-section">
    <h2 class="gallery-title">Gallery</h2>
    {#if images.length === 0 && !loadingMore}
      <div class="empty-state">
        <div class="empty-icon">
          <svg
            width="32"
            height="32"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="3" width="18" height="18" rx="2" /><circle
              cx="8.5"
              cy="8.5"
              r="1.5"
            /><path d="m21 15-5-5L5 21" />
          </svg>
        </div>
        <p class="empty-title">No images found</p>
        <p class="empty-sub">
          Images returned from <code>/images</code> will appear here
        </p>
      </div>
    {:else}
      <div class="gallery-grid">
        {#each images as img, i (img.src)}
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
          <div
            class="gallery-item"
            role="button"
            tabindex="0"
            onclick={() => openEditor(img)}
            onkeydown={(e) => e.key === 'Enter' && openEditor(img)}
            style="animation-delay:{Math.min(i, 12) * 40}ms"
          >
            {#if loadStates[img.src] === 'loading'}
              <div class="img-skeleton"></div>
            {:else if loadStates[img.src] === 'error'}
              <div class="img-error">
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.5"
                  ><circle cx="12" cy="12" r="10" /><line
                    x1="12"
                    y1="8"
                    x2="12"
                    y2="12"
                  /><line x1="12" y1="16" x2="12.01" y2="16" /></svg
                >
                <span>Failed</span>
              </div>
            {/if}
            <!-- THUMBNAIL: NO edit transforms — only the src/loading behaviour -->
            <img
              src={img.thumb}
              alt={img.name}
              class="gallery-img"
              class:loaded={loadStates[img.src] === 'loaded'}
              onload={() => onLoad(img.src)}
              onerror={(e) => {
                const el = e.currentTarget as HTMLImageElement;
                if (el.src !== img.src) {
                  el.src = img.src;
                } else {
                  onError(img.src);
                }
              }}
              loading="lazy"
            />
            {#if img.date}
              <div class="date-badge">{formatDate(img.date)}</div>
            {/if}
            {#if img.resolution}
              <div class="res-badge">{img.resolution}</div>
            {/if}
            {#if allEdits[img.src] && isEdited(allEdits[img.src])}
              <div class="edited-badge">edited</div>
            {/if}
            <div class="gallery-overlay">
              <div class="overlay-actions">
                <div class="overlay-icon">
                  <svg
                    width="15"
                    height="15"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path
                      d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"
                    />
                    <path
                      d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"
                    />
                  </svg>
                </div>
                <button
                  class="overlay-icon dl-btn"
                  title="Download"
                  onclick={(e) => download(img, e)}
                >
                  <svg
                    width="15"
                    height="15"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path
                      d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"
                    /><polyline points="7 10 12 15 17 10" /><line
                      x1="12"
                      y1="15"
                      x2="12"
                      y2="3"
                    />
                  </svg>
                </button>
                <button
                  class="overlay-icon del-btn"
                  title="Delete"
                  onclick={(e) => {
                    e.stopPropagation();
                    deleteTarget = img;
                  }}
                >
                  <svg
                    width="15"
                    height="15"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="3 6 5 6 21 6" /><path
                      d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"
                    />
                    <path d="M10 11v6" /><path d="M14 11v6" /><path
                      d="M9 6V4h6v2"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        {/each}
      </div>
      <div bind:this={sentinel} class="sentinel">
        {#if loadingMore}
          <div class="load-spinner">
            <div class="spinner"></div>
            <span>Loading more…</span>
          </div>
        {:else if !hasMore && images.length > 0}
          <p class="end-label">All {images.length} images loaded</p>
        {/if}
      </div>
    {/if}
  </section>
</main>

<!-- ═══════════════════════════════ EDITOR ════════════════════════════════════ -->
{#if editorOpen && editorImage}
  <div class="editor-backdrop">
    <!-- ── Canvas area (center, always fills remaining space) ── -->
    <div class="editor-canvas" class:crop-mode={cropMode}>
      <!-- Editor topbar -->
      <div class="editor-topbar">
        <button class="ed-top-btn" onclick={closeEditor} title="Close (Esc)">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <line x1="18" y1="6" x2="6" y2="18" /><line
              x1="6"
              y1="6"
              x2="18"
              y2="18"
            />
          </svg>
          Close
        </button>
        <div class="ed-title-block">
          <span class="ed-filename"
            >{editorImage.name.replace(/^VRChat_/, '')}</span
          >
          {#if editorImage.date}
            <span class="ed-date">{formatDate(editorImage.date)}</span>
          {/if}
        </div>
        <div class="ed-top-right">
          {#if editorHasEdits}
            <span class="ed-autosave-tag">auto-saved</span>
          {/if}
          <button
            class="ed-top-btn"
            onclick={(e) => download(editorImage!, e, edit)}
            title="Download (Ctrl+D)"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline
                points="7 10 12 15 17 10"
              /><line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            Download
          </button>
          <button
            class="ed-top-btn panel-toggle"
            onclick={() => (sidebarOpen = !sidebarOpen)}
            title="Toggle panel (P)"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              {#if sidebarOpen}
                <rect x="3" y="3" width="18" height="18" rx="2" /><line
                  x1="15"
                  y1="3"
                  x2="15"
                  y2="21"
                />
              {:else}
                <rect x="3" y="3" width="18" height="18" rx="2" /><line
                  x1="9"
                  y1="3"
                  x2="9"
                  y2="21"
                />
              {/if}
            </svg>
            {sidebarOpen ? 'Hide panel' : 'Show panel'}
          </button>
        </div>
      </div>

      <!-- Image stage -->
      <div class="canvas-stage">
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
        <div
          class="crop-zone"
          onpointerdown={cropMode ? onCropDown : undefined}
          onpointermove={cropMode ? onCropMove : undefined}
          onpointerup={cropMode ? onCropUp : undefined}
        >
          <img
            src={editorImage.src}
            alt={editorImage.name}
            class="editor-img"
            style="filter:{editorFilter};transform:{editorTransform};"
          />
          {#if edit.vignette > 0}
            <div
              class="vignette-overlay"
              style="--va:{edit.vignette / 100}"
            ></div>
          {/if}
          {#if edit.crop && !cropMode}
            <div
              class="crop-mask"
              style="--cx:{edit.crop.x}%;--cy:{edit.crop.y}%;--cw:{edit.crop
                .w}%;--ch:{edit.crop.h}%"
            ></div>
          {/if}
          {#if cropDraft && cropDraft.w > 0.5 && cropDraft.h > 0.5}
            <div
              class="crop-rect"
              style="left:{cropDraft.x}%;top:{cropDraft.y}%;width:{cropDraft.w}%;height:{cropDraft.h}%"
            >
              <div class="cc tl"></div>
              <div class="cc tr"></div>
              <div class="cc bl"></div>
              <div class="cc br"></div>
            </div>
          {/if}
        </div>

        <!-- Crop mode floating bar -->
        {#if cropMode}
          <div class="crop-bar">
            <span class="crop-hint">Drag to select region</span>
            <button
              class="crop-btn apply"
              onclick={applyCrop}
              disabled={!cropDraft || cropDraft.w < 0.5}>Apply</button
            >
            <button class="crop-btn cancel" onclick={cancelCrop}>Cancel</button>
          </div>
        {/if}

        <!-- Resolution chip -->
        {#if editorImage.resolution}
          <div class="stage-chip">{editorImage.resolution}</div>
        {/if}
      </div>
    </div>

    <!-- ── Sidebar ── -->
    {#if sidebarOpen}
      <aside class="tool-panel">
        <div class="tp-header">
          <span class="tp-title">Tools</span>
          <span class="tp-keys"
            ><kbd>r</kbd> rotate · <kbd>c</kbd> crop · <kbd>p</kbd> panel</span
          >
        </div>

        <div class="tp-body">
          <!-- Transform -->
          <div class="tp-section">
            <div class="tp-section-label">Transform</div>
            <div class="tp-row">
              <button class="tb" onclick={() => rotate(-90)}>
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path
                    d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"
                  /><path d="M3 3v5h5" />
                </svg>
                CCW
              </button>
              <button class="tb" onclick={() => rotate(90)}>
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path
                    d="M21 12a9 9 0 1 1-9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"
                  /><path d="M21 3v5h-5" />
                </svg>
                CW
              </button>
              <button
                class="tb"
                class:tb-active={edit.flipH}
                onclick={toggleFlipH}>⇔ Flip H</button
              >
              <button
                class="tb"
                class:tb-active={edit.flipV}
                onclick={toggleFlipV}>⇕ Flip V</button
              >
            </div>
            {#if edit.rotation !== 0 || edit.flipH || edit.flipV}
              <div class="tp-chip">
                {edit.rotation}°{edit.flipH ? ' ⇔' : ''}{edit.flipV ? ' ⇕' : ''}
              </div>
            {/if}
          </div>

          <!-- Crop -->
          <div class="tp-section">
            <div class="tp-section-label">Crop</div>
            <div class="tp-row">
              <button
                class="tb tb-wide"
                class:tb-active={cropMode}
                onclick={enterCropMode}
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M6 2v14a2 2 0 0 0 2 2h14" /><path
                    d="M18 22V8a2 2 0 0 0-2-2H2"
                  />
                </svg>
                {cropMode ? 'Drawing crop…' : 'Draw crop'}
              </button>
              {#if edit.crop}
                <button class="tb tb-danger" onclick={resetCrop}>✕ Reset</button
                >
              {/if}
            </div>
            {#if edit.crop}
              <div class="tp-chip">
                {edit.crop.x.toFixed(0)},{edit.crop.y.toFixed(0)} · {edit.crop.w.toFixed(
                  0,
                )}×{edit.crop.h.toFixed(0)}%
              </div>
            {/if}
          </div>

          <!-- Adjustments -->
          <div class="tp-section">
            <div class="tp-section-label">Adjustments</div>
            {#each [{ key: 'brightness', label: 'Brightness', min: 0, max: 200, def: 100 }, { key: 'contrast', label: 'Contrast', min: 0, max: 200, def: 100 }, { key: 'saturation', label: 'Saturation', min: 0, max: 200, def: 100 }, { key: 'hue', label: 'Hue', min: 0, max: 360, def: 0 }, { key: 'vignette', label: 'Vignette', min: 0, max: 100, def: 0 }] as adj}
              <div class="adj-row">
                <div class="adj-head">
                  <span class="adj-label">{adj.label}</span>
                  <span class="adj-val"
                    >{edit[adj.key as keyof EditState] as number}</span
                  >
                  {#if (edit[adj.key as keyof EditState] as number) !== adj.def}
                    <button
                      class="adj-reset"
                      title="Reset"
                      onclick={() => (edit = { ...edit, [adj.key]: adj.def })}
                      >↺</button
                    >
                  {/if}
                </div>
                <div class="adj-slider-wrap">
                  <div class="adj-center-tick"></div>
                  <input
                    type="range"
                    min={adj.min}
                    max={adj.max}
                    step="1"
                    value={edit[adj.key as keyof EditState] as number}
                    style="--pct:{(((edit[
                      adj.key as keyof EditState
                    ] as number) -
                      adj.min) /
                      (adj.max - adj.min)) *
                      100}%"
                    oninput={(e) =>
                      (edit = {
                        ...edit,
                        [adj.key]: Number(
                          (e.currentTarget as HTMLInputElement).value,
                        ),
                      })}
                    class="adj-slider"
                  />
                </div>
              </div>
            {/each}
          </div>
        </div>
        <!-- tp-body -->

        <!-- Footer -->
        <div class="tp-footer">
          <button
            class="tf-btn"
            onclick={() => {
              edit = { ...DEFAULT_EDIT };
            }}>Reset all</button
          >
          {#if editorHasEdits}
            <button
              class="tf-btn tf-danger"
              onclick={() => {
                clearEdit(editorImage!.src);
                edit = { ...DEFAULT_EDIT };
              }}>Discard saved</button
            >
          {/if}
        </div>
      </aside>
    {/if}
  </div>
  <!-- editor-backdrop -->
{/if}

<!-- Delete dialog -->
{#if deleteTarget}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
  <div
    class="dialog-bd"
    onclick={() => (deleteTarget = null)}
    role="dialog"
    aria-modal="true"
  >
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
    <div class="dialog" onclick={(e) => e.stopPropagation()}>
      <div class="dlg-icon">
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          ><circle cx="12" cy="12" r="10" /><line
            x1="12"
            y1="8"
            x2="12"
            y2="12"
          /><line x1="12" y1="16" x2="12.01" y2="16" /></svg
        >
      </div>
      <h3 class="dlg-title">Delete image?</h3>
      <p class="dlg-sub">{deleteTarget.name}</p>
      <div class="dlg-actions">
        <button
          class="dlg-btn dlg-cancel"
          onclick={() => (deleteTarget = null)}
          disabled={deleting}>Cancel</button
        >
        <button
          class="dlg-btn dlg-confirm"
          onclick={confirmDelete}
          disabled={deleting}>{deleting ? 'Deleting…' : 'Delete'}</button
        >
      </div>
    </div>
  </div>
{/if}

<style>
  /* ── Reset / base ─────────────────────────────────────────────────────────── */
  @keyframes fadeUp {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
  @keyframes shimmer {
    0% {
      background-position: 200% 0;
    }
    100% {
      background-position: -200% 0;
    }
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @keyframes fadein {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  /* ── Header ──────────────────────────────────────────────────────────────── */
  .header {
    position: sticky;
    top: 0;
    z-index: 50;
    background: color-mix(in srgb, var(--color-bg) 80%, transparent);
    backdrop-filter: blur(16px);
    border-bottom: 1px solid var(--color-border);
  }
  .header-inner {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 2rem;
    height: 56px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .logo {
    display: flex;
    align-items: center;
    gap: 0.625rem;
  }
  .logo-icon {
    width: 32px;
    height: 32px;
    background: var(--color-accent-glow);
    border: 1px solid var(--color-accent);
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent);
  }
  .logo-text {
    font-size: 0.9375rem;
    font-weight: 600;
    letter-spacing: -0.02em;
  }
  .image-count {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  /* ── Main / gallery ──────────────────────────────────────────────────────── */
  .main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2.5rem 2rem 4rem;
    display: flex;
    flex-direction: column;
    gap: 2.5rem;
  }

  .query-panel {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    overflow: hidden;
  }
  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface-2);
  }
  .panel-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-text-muted);
  }
  .sync-indicator {
    display: flex;
    align-items: center;
    gap: 0.375rem;
  }
  .sync-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }
  .sync-dot.idle {
    background: var(--color-border-hover);
  }
  .sync-dot.syncing {
    background: var(--color-syncing);
    animation: pulse-dot 0.9s ease-in-out infinite;
  }
  .sync-dot.done {
    background: var(--color-success);
  }
  .sync-dot.error {
    background: #f87171;
  }
  .sync-label {
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }
  @keyframes pulse-dot {
    0%,
    100% {
      opacity: 1;
      transform: scale(1);
    }
    50% {
      opacity: 0.4;
      transform: scale(0.7);
    }
  }

  .query-textarea {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    resize: none;
    padding: 1rem 1.25rem;
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.7;
    color: var(--color-text);
    caret-color: var(--color-accent);
    display: block;
  }
  .query-textarea::placeholder {
    color: var(--color-text-muted);
  }
  .textarea-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.5rem 1.25rem 0.75rem;
    border-top: 1px solid var(--color-border);
  }
  .char-count {
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }
  .clear-btn {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    background: none;
    border: none;
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    transition:
      color 0.15s,
      background 0.15s;
  }
  .clear-btn:hover {
    color: var(--color-text);
    background: var(--color-border);
  }

  .gallery-section {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .gallery-title {
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    margin: 0;
  }

  .gallery-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 1px;
    background: var(--color-border);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    overflow: hidden;
  }
  .gallery-item {
    position: relative;
    background: var(--color-surface);
    aspect-ratio: 16/10;
    overflow: hidden;
    cursor: pointer;
    animation: fadeUp 0.35s ease both;
  }
  .gallery-item:hover .gallery-overlay {
    opacity: 1;
  }
  .gallery-item:hover .gallery-img {
    transform: scale(1.04);
  }

  .gallery-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    opacity: 0;
    transition:
      opacity 0.3s ease,
      transform 0.4s ease;
  }
  .gallery-img.loaded {
    opacity: 1;
  }

  .img-skeleton {
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      var(--color-surface) 25%,
      var(--color-surface-2) 50%,
      var(--color-surface) 75%
    );
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
  }
  .img-error {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-family: var(--font-mono);
  }

  .date-badge {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 1.5rem 0.625rem 0.4rem;
    background: linear-gradient(
      to top,
      rgba(0, 0, 0, 0.72) 0%,
      transparent 100%
    );
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: rgba(255, 255, 255, 0.85);
    pointer-events: none;
  }
  .res-badge {
    position: absolute;
    top: 0.4rem;
    right: 0.4rem;
    background: rgba(0, 0, 0, 0.55);
    backdrop-filter: blur(4px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 4px;
    padding: 0.1rem 0.35rem;
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: rgba(255, 255, 255, 0.6);
    pointer-events: none;
  }
  .edited-badge {
    position: absolute;
    top: 0.4rem;
    left: 0.4rem;
    background: color-mix(in srgb, var(--color-accent) 80%, transparent);
    border-radius: 4px;
    padding: 0.1rem 0.4rem;
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: #fff;
    pointer-events: none;
  }

  .gallery-overlay {
    position: absolute;
    inset: 0;
    background: color-mix(in srgb, var(--color-accent) 15%, #0008);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.2s;
  }
  .overlay-actions {
    display: flex;
    gap: 0.4rem;
  }
  .overlay-icon {
    width: 36px;
    height: 36px;
    background: color-mix(in srgb, var(--color-bg) 55%, transparent);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    backdrop-filter: blur(4px);
    cursor: default;
  }
  .dl-btn,
  .del-btn {
    cursor: pointer;
    padding: 0;
    border: 1px solid rgba(255, 255, 255, 0.15);
    background: color-mix(in srgb, var(--color-bg) 55%, transparent);
    border-radius: 8px;
    transition:
      background 0.15s,
      border-color 0.15s;
  }
  .dl-btn:hover {
    background: color-mix(in srgb, var(--color-accent) 55%, transparent);
    border-color: var(--color-accent);
  }
  .del-btn:hover {
    background: color-mix(in srgb, #ef4444 55%, transparent);
    border-color: #ef4444;
  }

  .sentinel {
    padding: 2rem 0;
    display: flex;
    justify-content: center;
  }
  .load-spinner {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    color: var(--color-text-muted);
    font-size: 0.8125rem;
    font-family: var(--font-mono);
  }
  .spinner {
    width: 18px;
    height: 18px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  .end-label {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    margin: 0;
  }

  .empty-state {
    padding: 5rem 2rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 0.75rem;
    background: var(--color-surface);
    border: 1px dashed var(--color-border);
    border-radius: 12px;
  }
  .empty-icon {
    width: 56px;
    height: 56px;
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-muted);
    margin-bottom: 0.5rem;
  }
  .empty-title {
    font-size: 0.9375rem;
    font-weight: 500;
    color: var(--color-text-dim);
    margin: 0;
  }
  .empty-sub {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
    margin: 0;
  }
  .empty-sub code {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    background: var(--color-surface-2);
    padding: 0.1em 0.4em;
    border-radius: 4px;
    border: 1px solid var(--color-border);
  }

  /* ══════════════════════════════ EDITOR ══════════════════════════════════════ */
  .editor-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    background: var(--color-bg);
    animation: fadein 0.15s ease;
  }

  /* Canvas side */
  .editor-canvas {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    background: #0d0e13;
  }
  .editor-canvas.crop-mode {
    cursor: crosshair;
  }

  /* Topbar */
  .editor-topbar {
    height: 48px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0 1rem;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface);
  }
  .ed-top-btn {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    border: 1px solid var(--color-border);
    background: transparent;
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-family: var(--font-mono);
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s,
      background 0.15s;
    flex-shrink: 0;
  }
  .ed-top-btn:hover {
    color: var(--color-text);
    border-color: var(--color-border-hover);
    background: var(--color-surface-2);
  }
  .panel-toggle {
    margin-left: auto;
  }

  .ed-title-block {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    flex: 1;
    min-width: 0;
    overflow: hidden;
  }
  .ed-filename {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ed-date {
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .ed-top-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
  }
  .ed-autosave-tag {
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: var(--color-success);
    background: color-mix(in srgb, var(--color-success) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-success) 30%, transparent);
    border-radius: 4px;
    padding: 0.1rem 0.4rem;
  }

  /* Stage */
  .canvas-stage {
    flex: 1;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    container-type: size;
  }
  .crop-zone {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .editor-img {
    display: block;
    /* Both axes clamped to the smaller of stage width/height.
     When rotated 90/270°, CSS width becomes visual height — so by limiting
     to min(cqw,cqh) on BOTH axes the image always fits the stage. */
    max-width: min(100cqw, 100cqh);
    max-height: min(100cqh, 100cqw);
    width: auto;
    height: auto;
    object-fit: contain;
    border-radius: 2px;
    transition:
      filter 0.2s ease,
      transform 0.25s ease;
    transform-origin: center center;
  }
  .vignette-overlay {
    position: absolute;
    inset: 0;
    pointer-events: none;
    border-radius: 2px;
    background: radial-gradient(
      ellipse at center,
      rgba(0, 0, 0, 0) 25%,
      rgba(0, 0, 0, calc(var(--va, 0) * 1)) 100%
    );
  }
  .crop-mask {
    position: absolute;
    inset: 0;
    pointer-events: none;
    box-shadow:
      0 0 0 9999px rgba(0, 0, 0, 0.55),
      inset calc(var(--cx)) calc(var(--cy)) 0 0 rgba(0, 0, 0, 0);
    /* Use clip-path to show only the crop region */
    clip-path: polygon(
      0% 0%,
      0% 100%,
      var(--cx) 100%,
      var(--cx) var(--cy),
      calc(var(--cx) + var(--cw)) var(--cy),
      calc(var(--cx) + var(--cw)) calc(var(--cy) + var(--ch)),
      var(--cx) calc(var(--cy) + var(--ch)),
      var(--cx) 100%,
      100% 100%,
      100% 0%
    );
    background: rgba(0, 0, 0, 0.55);
  }
  .crop-rect {
    position: absolute;
    border: 1.5px solid #fff;
    background: rgba(255, 255, 255, 0.06);
    pointer-events: none;
  }
  .cc {
    position: absolute;
    width: 8px;
    height: 8px;
    background: #fff;
    border-radius: 1px;
  }
  .cc.tl {
    top: -2px;
    left: -2px;
  }
  .cc.tr {
    top: -2px;
    right: -2px;
  }
  .cc.bl {
    bottom: -2px;
    left: -2px;
  }
  .cc.br {
    bottom: -2px;
    right: -2px;
  }

  .crop-bar {
    position: absolute;
    bottom: 1.25rem;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: color-mix(in srgb, var(--color-surface) 95%, transparent);
    border: 1px solid var(--color-border);
    border-radius: 9px;
    padding: 0.45rem 0.875rem;
    backdrop-filter: blur(8px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }
  .crop-hint {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }
  .crop-btn {
    padding: 0.25rem 0.7rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-family: var(--font-mono);
    cursor: pointer;
    border: 1px solid;
    transition: all 0.15s;
  }
  .crop-btn.apply {
    background: var(--color-accent);
    border-color: var(--color-accent);
    color: #fff;
  }
  .crop-btn.apply:hover:not(:disabled) {
    background: var(--color-accent-dim);
    border-color: var(--color-accent-dim);
  }
  .crop-btn.apply:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .crop-btn.cancel {
    background: var(--color-surface-2);
    border-color: var(--color-border);
    color: var(--color-text-muted);
  }
  .crop-btn.cancel:hover {
    color: var(--color-text);
    border-color: var(--color-border-hover);
  }

  .stage-chip {
    position: absolute;
    bottom: 0.875rem;
    right: 0.875rem;
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: rgba(255, 255, 255, 0.35);
    background: rgba(0, 0, 0, 0.4);
    border-radius: 4px;
    padding: 0.15rem 0.4rem;
    pointer-events: none;
  }

  /* ── Tool panel ──────────────────────────────────────────────────────────── */
  .tool-panel {
    width: 272px;
    flex-shrink: 0;
    background: var(--color-surface);
    border-left: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .tp-header {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface-2);
    flex-shrink: 0;
  }
  .tp-title {
    font-size: 0.8125rem;
    font-weight: 600;
    display: block;
    margin-bottom: 0.2rem;
  }
  .tp-keys {
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }
  .tp-keys kbd {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 3px;
    padding: 0 0.25em;
  }
  .tp-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem 0;
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }
  .tp-section {
    padding: 0.625rem 1rem;
  }
  .tp-section-label {
    font-size: 0.625rem;
    font-weight: 500;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    margin-bottom: 0.5rem;
  }
  .tp-row {
    display: flex;
    gap: 0.3rem;
    flex-wrap: wrap;
  }
  .tp-chip {
    font-size: 0.625rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    margin-top: 0.35rem;
  }

  /* Tool buttons */
  .tb {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    color: var(--color-text-dim);
    font-size: 0.75rem;
    font-family: var(--font-mono);
    cursor: pointer;
    transition: all 0.15s;
  }
  .tb:hover {
    background: var(--color-border);
    color: var(--color-text);
    border-color: var(--color-border-hover);
  }
  .tb-active {
    background: color-mix(
      in srgb,
      var(--color-accent) 15%,
      transparent
    ) !important;
    border-color: var(--color-accent) !important;
    color: var(--color-accent) !important;
  }
  .tb-wide {
    flex: 1;
    justify-content: center;
  }
  .tb-danger {
    color: #f87171;
    border-color: color-mix(in srgb, #ef4444 35%, transparent);
  }
  .tb-danger:hover {
    background: color-mix(in srgb, #ef4444 12%, transparent) !important;
    border-color: #ef4444 !important;
  }

  /* Sliders */
  .adj-row {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    padding: 0.5rem 0.875rem;
    border-radius: 8px;
    background: var(--color-surface-2);
    border: 1px solid var(--color-border);
    margin-bottom: 0.375rem;
    transition: border-color 0.15s;
  }
  .adj-row:hover {
    border-color: var(--color-border-hover);
  }
  .adj-head {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    margin-bottom: 0.1rem;
  }
  .adj-label {
    font-size: 0.6875rem;
    font-weight: 500;
    color: var(--color-text-dim);
    flex: 1;
    letter-spacing: 0.01em;
  }
  .adj-val {
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
    border-radius: 4px;
    padding: 0.05rem 0.35rem;
    min-width: 2.2rem;
    text-align: center;
  }
  .adj-reset {
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: 1rem;
    padding: 0 0.15rem;
    line-height: 1;
    transition:
      color 0.15s,
      transform 0.15s;
  }
  .adj-reset:hover {
    color: var(--color-accent);
    transform: rotate(-30deg);
  }

  /* Slider track wrapper for center tick */
  .adj-slider-wrap {
    position: relative;
    padding: 0.25rem 0;
  }
  .adj-center-tick {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 1px;
    height: 10px;
    background: var(--color-border-hover);
    pointer-events: none;
    border-radius: 1px;
    margin-top: 1px; /* optical center of track */
  }
  .adj-slider {
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
    height: 5px;
    border-radius: 3px;
    outline: none;
    cursor: pointer;
    display: block;
    background: linear-gradient(
      to right,
      var(--color-accent) var(--pct, 50%),
      color-mix(in srgb, var(--color-border-hover) 60%, var(--color-surface))
        var(--pct, 50%)
    );
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  }
  .adj-slider:focus {
    outline: none;
  }
  .adj-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--color-text);
    border: 2.5px solid var(--color-accent);
    cursor: pointer;
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--color-accent) 30%, transparent);
    transition:
      box-shadow 0.15s,
      transform 0.1s;
  }
  .adj-slider::-webkit-slider-thumb:hover {
    box-shadow: 0 0 0 5px
      color-mix(in srgb, var(--color-accent) 20%, transparent);
    transform: scale(1.1);
  }
  .adj-slider:active::-webkit-slider-thumb {
    box-shadow: 0 0 0 7px
      color-mix(in srgb, var(--color-accent) 15%, transparent);
    transform: scale(1.15);
  }
  .adj-slider::-moz-range-thumb {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--color-text);
    border: 2.5px solid var(--color-accent);
    cursor: pointer;
    box-shadow: none;
  }
  .adj-slider::-moz-range-track {
    height: 5px;
    border-radius: 3px;
    background: color-mix(
      in srgb,
      var(--color-border-hover) 60%,
      var(--color-surface)
    );
  }

  /* Footer */
  .tp-footer {
    padding: 0.75rem 1rem;
    border-top: 1px solid var(--color-border);
    display: flex;
    gap: 0.375rem;
    flex-shrink: 0;
    background: var(--color-surface-2);
  }
  .tf-btn {
    padding: 0.35rem 0.7rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-family: var(--font-mono);
    cursor: pointer;
    border: 1px solid var(--color-border);
    background: transparent;
    color: var(--color-text-muted);
    transition: all 0.15s;
  }
  .tf-btn:hover {
    color: var(--color-text);
    border-color: var(--color-border-hover);
    background: var(--color-surface);
  }
  .tf-danger {
    color: #f87171;
    border-color: color-mix(in srgb, #ef4444 35%, transparent);
  }
  .tf-danger:hover {
    color: #f87171;
    border-color: #ef4444;
    background: color-mix(in srgb, #ef4444 8%, transparent) !important;
  }

  /* ── Delete dialog ────────────────────────────────────────────────────────── */
  .dialog-bd {
    position: fixed;
    inset: 0;
    z-index: 200;
    background: rgba(0, 0, 0, 0.72);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    animation: fadein 0.15s ease;
  }
  .dialog {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 14px;
    padding: 2rem;
    width: min(420px, calc(100vw - 2rem));
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.625rem;
    text-align: center;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.5);
  }
  .dlg-icon {
    width: 46px;
    height: 46px;
    border-radius: 12px;
    background: color-mix(in srgb, #ef4444 10%, transparent);
    border: 1px solid color-mix(in srgb, #ef4444 28%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #f87171;
    margin-bottom: 0.25rem;
  }
  .dlg-title {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
  }
  .dlg-sub {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    margin: 0;
    word-break: break-all;
  }
  .dlg-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.625rem;
    width: 100%;
  }
  .dlg-btn {
    flex: 1;
    padding: 0.5rem 1rem;
    border-radius: 8px;
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    border: 1px solid var(--color-border);
    transition: all 0.15s;
  }
  .dlg-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .dlg-cancel {
    background: var(--color-surface-2);
    color: var(--color-text-dim);
  }
  .dlg-cancel:hover:not(:disabled) {
    border-color: var(--color-border-hover);
    color: var(--color-text);
  }
  .dlg-confirm {
    background: #ef4444;
    border-color: #ef4444;
    color: #fff;
  }
  .dlg-confirm:hover:not(:disabled) {
    background: #dc2626;
    border-color: #dc2626;
  }

  /* ── Responsive ──────────────────────────────────────────────────────────── */
  @media (max-width: 768px) {
    .main {
      padding: 1.5rem 1rem 3rem;
      gap: 1.5rem;
    }
    .header-inner {
      padding: 0 1rem;
    }
    .gallery-grid {
      grid-template-columns: repeat(2, 1fr);
    }
    .editor-backdrop {
      flex-direction: column;
    }
    .tool-panel {
      width: 100%;
      height: 260px;
      border-left: none;
      border-top: 1px solid var(--color-border);
    }
    .tp-body {
      flex-direction: row;
      overflow-x: auto;
      padding: 0.5rem;
    }
    .tp-section {
      min-width: 180px;
    }
    .editor-topbar {
      gap: 0.4rem;
    }
    .ed-title-block {
      display: none;
    }
  }
</style>
