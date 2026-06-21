<script lang="ts">
  import { onMount } from 'svelte';

  // ── Types ────────────────────────────────────────────────────────────────────
  interface ImageFile {
    src: string; // original path e.g. "images/VRChat_2026-06-21_01-34-21.865_2048x1440.png"
    thumb: string; // thumbnail path (same but with ?w=400 hint — server decides)
    name: string;
    date: Date | null;
    resolution: string | null;
    id: string; // sha256 of filename sent by server
  }

  // ── State ────────────────────────────────────────────────────────────────────
  let query = $state('');
  let images = $state<ImageFile[]>([]);
  let loadStates = $state<Record<string, 'loading' | 'loaded' | 'error'>>({});
  let syncStatus = $state<'idle' | 'syncing' | 'done' | 'error'>('idle');
  let selected = $state<ImageFile | null>(null);
  let charCount = $derived(query.length);

  // Pagination
  let page = $state(1);
  let hasMore = $state(true);
  let loadingMore = $state(false);
  let sentinel: HTMLDivElement;

  // Delete confirm
  let deleteTarget = $state<ImageFile | null>(null);
  let deleting = $state(false);

  // ── Filename parsing ─────────────────────────────────────────────────────────
  // Handles: VRChat_2026-06-21_01-34-21.865_2048x1440.png
  //          VRChat_2025-11-03_22-11-05.123_1920x1080.png
  function parseFilename(
    path: string,
  ): Pick<ImageFile, 'date' | 'resolution' | 'name'> {
    const name = path.split('/').pop() ?? path;
    // Match YYYY-MM-DD_HH-MM-SS
    const dateMatch = name.match(
      /(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})/,
    );
    // Match WxH resolution
    const resMatch = name.match(/(\d{3,5}x\d{3,5})/);
    let date: Date | null = null;
    if (dateMatch) {
      const [, y, mo, d, h, mi, s] = dateMatch;
      date = new Date(`${y}-${mo}-${d}T${h}:${mi}:${s}`);
    }
    return {
      name,
      date,
      resolution: resMatch ? resMatch[1] : null,
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

  // ── Data fetching ─────────────────────────────────────────────────────────────
  // Server returns: { files: string[], total?: number }
  // OR extended:    { files: [{src, id}], total }
  // We handle both shapes.
  // ── Thumbnail path ────────────────────────────────────────────────────────────
  // "images/VRChat_....png" → "images/thumbnails/VRChat_....png"
  function toThumb(src: string): string {
    const parts = src.split('/');
    // insert "thumbnails" before the filename
    parts.splice(parts.length - 1, 0, 'thumbnails');
    const t = parts.join('/');
    console.log({ t });
    return t;
  }

  function normalise(raw: string | { src: string; id?: string }): {
    src: string;
    id: string;
  } {
    if (typeof raw === 'string') {
      return { src: raw, id: btoa(raw) };
    }
    return { src: raw.src, id: raw.id ?? btoa(raw.src) };
  }

  async function fetchPage(p: number) {
    const res = await fetch(`/images?page=${p}`);
    if (!res.ok) throw new Error('fetch failed');
    const json = await res.json();
    const rawFiles: (string | { src: string; id?: string })[] =
      json.files ?? [];
    const total: number = json.total ?? Infinity;

    const newImages: ImageFile[] = rawFiles.map((raw) => {
      const { src, id } = normalise(raw);
      const parsed = parseFilename(src);
      const thumb = toThumb(src); // images/thumbnails/filename.png — falls back to src on 404
      return { src, thumb, id, ...parsed };
    });

    return { newImages, total };
  }

  async function loadMore() {
    if (loadingMore || !hasMore) return;
    loadingMore = true;
    try {
      const { newImages, total } = await fetchPage(page);
      for (const img of newImages) loadStates[img.src] = 'loading';
      images = [...images, ...newImages];
      hasMore = images.length < total && newImages.length > 0;
      page += 1;
    } catch {
      hasMore = false;
    } finally {
      loadingMore = false;
    }
  }

  onMount(async () => {
    await loadMore();

    // Infinite scroll via IntersectionObserver on sentinel div
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) loadMore();
      },
      { rootMargin: '200px' },
    );

    if (sentinel) observer.observe(sentinel);
    return () => observer.disconnect();
  });

  // ── Auto-sync ─────────────────────────────────────────────────────────────────
  let debounceTimer: ReturnType<typeof setTimeout>;

  function scheduleSync(value: string) {
    syncStatus = 'syncing';
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      fetch('http://localhost:3001/copy', {
        method: 'POST',
        body: JSON.stringify({ query: value }),
        headers: { 'content-type': 'application/json' },
      })
        .then((res) => {
          syncStatus = res.ok ? 'done' : 'error';
          setTimeout(() => (syncStatus = 'idle'), 2000);
        })
        .catch(() => {
          syncStatus = 'error';
          setTimeout(() => (syncStatus = 'idle'), 2000);
        });
    }, 500);
  }

  function handleInput(e: Event) {
    const value = (e.currentTarget as HTMLTextAreaElement).value;
    query = value;
    scheduleSync(value);
  }

  function clearQuery() {
    query = '';
    scheduleSync('');
  }

  // ── Image load state ──────────────────────────────────────────────────────────
  function onLoad(src: string) {
    loadStates[src] = 'loaded';
  }
  function onError(src: string) {
    loadStates[src] = 'error';
  }

  // ── Download ──────────────────────────────────────────────────────────────────
  async function download(img: ImageFile, e: MouseEvent) {
    e.stopPropagation();
    try {
      const res = await fetch(img.src);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = img.name;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      window.open(img.src, '_blank');
    }
  }

  // ── Delete ────────────────────────────────────────────────────────────────────
  async function confirmDelete() {
    if (!deleteTarget) return;
    deleting = true;
    try {
      const res = await fetch(`/images/${deleteTarget.id}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        images = images.filter((i) => i.src !== deleteTarget!.src);
        if (selected?.src === deleteTarget.src) selected = null;
        deleteTarget = null;
      }
    } finally {
      deleting = false;
    }
  }
</script>

<!-- ── Header ──────────────────────────────────────────────────────────────── -->
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
    <div class="header-meta">
      <span class="image-count"
        >{images.length} image{images.length !== 1 ? 's' : ''}{hasMore
          ? '+'
          : ''}</span
      >
    </div>
  </div>
</header>

<main class="main">
  <!-- ── Query Panel ─────────────────────────────────────────────────────────── -->
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
    <div class="textarea-wrapper">
      <textarea
        value={query}
        oninput={handleInput}
        rows={5}
        placeholder="Type your query here… auto-synced after 500ms"
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
    </div>
  </section>

  <!-- ── Gallery ────────────────────────────────────────────────────────────── -->
  <section class="gallery-section">
    <div class="gallery-header">
      <h2 class="gallery-title">Gallery</h2>
    </div>

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
            onclick={() => (selected = img)}
            onkeydown={(e) => e.key === 'Enter' && (selected = img)}
            style="animation-delay: {Math.min(i, 12) * 40}ms"
          >
            {#if loadStates[img.src] === 'loading'}
              <div class="img-skeleton"></div>
            {:else if loadStates[img.src] === 'error'}
              <div class="img-error">
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <circle cx="12" cy="12" r="10" /><line
                    x1="12"
                    y1="8"
                    x2="12"
                    y2="12"
                  /><line x1="12" y1="16" x2="12.01" y2="16" />
                </svg>
                <span>Failed</span>
              </div>
            {/if}

            <!-- Thumbnail for fast load; falls back to full src on 404 -->
            <img
              src={img.thumb}
              alt={img.name}
              class="gallery-img"
              class:loaded={loadStates[img.src] === 'loaded'}
              onload={() => onLoad(img.src)}
              onerror={(e) => {
                const el = e.currentTarget as HTMLImageElement;
                if (el.src !== img.src) {
                  el.src = img.src; // fallback to original
                } else {
                  onError(img.src);
                }
              }}
              loading="lazy"
            />

            <!-- Date badge -->
            {#if img.date}
              <div class="date-badge">{formatDate(img.date)}</div>
            {/if}

            <!-- Resolution badge -->
            {#if img.resolution}
              <div class="res-badge">{img.resolution}</div>
            {/if}

            <!-- Hover overlay -->
            <div class="gallery-overlay">
              <div class="overlay-actions">
                <div class="overlay-icon" title="View">
                  <svg
                    width="17"
                    height="17"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="m15 3h6v6" /><path d="m9 21h-6v-6" /><path
                      d="m21 3-7 7"
                    /><path d="m3 21 7-7" />
                  </svg>
                </div>
                <button
                  class="overlay-icon dl-btn"
                  title="Download"
                  onclick={(e) => download(img, e)}
                >
                  <svg
                    width="17"
                    height="17"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
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
                    width="17"
                    height="17"
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

      <!-- Infinite scroll sentinel + spinner -->
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

<!-- ── Lightbox ─────────────────────────────────────────────────────────────── -->
{#if selected}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
  <div
    class="lightbox"
    onclick={() => (selected = null)}
    role="dialog"
    aria-modal="true"
    aria-label="Image preview"
  >
    <div class="lightbox-actions">
      <button class="lightbox-btn" onclick={(e) => download(selected!, e)}>
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
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
        </svg>
        Download
      </button>
      <button
        class="lightbox-btn danger-btn"
        onclick={(e) => {
          e.stopPropagation();
          deleteTarget = selected;
        }}
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
          <polyline points="3 6 5 6 21 6" /><path
            d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"
          />
          <path d="M10 11v6" /><path d="M14 11v6" />
        </svg>
        Delete
      </button>
      <button class="lightbox-btn" onclick={() => (selected = null)}>
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
    </div>

    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
    <img
      src={selected.src}
      alt={selected.name}
      class="lightbox-img"
      onclick={(e) => e.stopPropagation()}
    />

    {#if selected.date || selected.resolution}
      <div class="lightbox-meta" onclick={(e) => e.stopPropagation()}>
        {#if selected.date}<span>{formatDate(selected.date)}</span>{/if}
        {#if selected.resolution}<span class="res-tag"
            >{selected.resolution}</span
          >{/if}
      </div>
    {/if}
  </div>
{/if}

<!-- ── Delete confirm dialog ─────────────────────────────────────────────────── -->
{#if deleteTarget}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
  <div
    class="dialog-backdrop"
    onclick={() => (deleteTarget = null)}
    role="dialog"
    aria-modal="true"
  >
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
    <div class="dialog" onclick={(e) => e.stopPropagation()}>
      <div class="dialog-icon">
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="12" r="10" /><line
            x1="12"
            y1="8"
            x2="12"
            y2="12"
          /><line x1="12" y1="16" x2="12.01" y2="16" />
        </svg>
      </div>
      <h3 class="dialog-title">Delete image?</h3>
      <p class="dialog-sub">{deleteTarget.name}</p>
      <div class="dialog-actions">
        <button
          class="dialog-btn cancel"
          onclick={() => (deleteTarget = null)}
          disabled={deleting}>Cancel</button
        >
        <button
          class="dialog-btn confirm"
          onclick={confirmDelete}
          disabled={deleting}
        >
          {deleting ? 'Deleting…' : 'Delete'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* ── Header ─────────────────────────────────────────────────────────────── */
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

  /* ── Main ────────────────────────────────────────────────────────────────── */
  .main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2.5rem 2rem 4rem;
    display: flex;
    flex-direction: column;
    gap: 2.5rem;
  }

  /* ── Query panel ─────────────────────────────────────────────────────────── */
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
    flex-shrink: 0;
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

  .textarea-wrapper {
    padding: 0;
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

  /* ── Gallery ─────────────────────────────────────────────────────────────── */
  .gallery-section {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .gallery-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
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
    animation: fadeIn 0.35s ease both;
  }
  .gallery-item:hover .gallery-overlay {
    opacity: 1;
  }
  .gallery-item:hover .gallery-img {
    transform: scale(1.04);
  }
  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
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
  @keyframes shimmer {
    0% {
      background-position: 200% 0;
    }
    100% {
      background-position: -200% 0;
    }
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

  /* Date + resolution badges */
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

  /* Overlay */
  .gallery-overlay {
    position: absolute;
    inset: 0;
    background: color-mix(in srgb, var(--color-accent) 15%, #0008);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.2s ease;
  }
  .overlay-actions {
    display: flex;
    gap: 0.45rem;
  }
  .overlay-icon {
    width: 38px;
    height: 38px;
    background: color-mix(in srgb, var(--color-bg) 55%, transparent);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 9px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    backdrop-filter: blur(4px);
    cursor: default;
  }
  .dl-btn,
  .del-btn {
    cursor: pointer;
    padding: 0;
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

  /* Sentinel / pagination */
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
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .end-label {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    margin: 0;
  }

  /* Empty state */
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

  /* ── Lightbox ────────────────────────────────────────────────────────────── */
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: rgba(0, 0, 0, 0.9);
    backdrop-filter: blur(14px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4rem 2rem 2rem;
    animation: fadeIn 0.15s ease;
  }
  .lightbox-actions {
    position: absolute;
    top: 1.25rem;
    right: 1.25rem;
    display: flex;
    gap: 0.5rem;
  }
  .lightbox-btn {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.4rem 0.75rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-family: var(--font-mono);
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s,
      background 0.15s;
  }
  .lightbox-btn:hover {
    color: var(--color-text);
    border-color: var(--color-border-hover);
    background: var(--color-surface-2);
  }
  .danger-btn:hover {
    color: #f87171;
    border-color: #ef4444;
  }
  .lightbox-img {
    max-width: 100%;
    max-height: 85vh;
    border-radius: 8px;
    box-shadow: 0 32px 80px rgba(0, 0, 0, 0.6);
    object-fit: contain;
  }
  .lightbox-meta {
    position: absolute;
    bottom: 1.5rem;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 0.75rem;
    background: color-mix(in srgb, var(--color-surface) 85%, transparent);
    backdrop-filter: blur(8px);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: 0.4rem 0.875rem;
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .res-tag {
    color: var(--color-accent);
    border-left: 1px solid var(--color-border);
    padding-left: 0.75rem;
  }

  /* ── Delete dialog ───────────────────────────────────────────────────────── */
  .dialog-backdrop {
    position: fixed;
    inset: 0;
    z-index: 200;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    animation: fadeIn 0.15s ease;
  }
  .dialog {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 14px;
    padding: 2rem;
    width: min(440px, calc(100vw - 2rem));
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.625rem;
    text-align: center;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.5);
  }
  .dialog-icon {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    background: color-mix(in srgb, #ef4444 12%, transparent);
    border: 1px solid color-mix(in srgb, #ef4444 30%, transparent);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #f87171;
    margin-bottom: 0.25rem;
  }
  .dialog-title {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
  }
  .dialog-sub {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    margin: 0;
    word-break: break-all;
  }
  .dialog-actions {
    display: flex;
    gap: 0.625rem;
    margin-top: 0.75rem;
    width: 100%;
  }
  .dialog-btn {
    flex: 1;
    padding: 0.55rem 1rem;
    border-radius: 8px;
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    border: 1px solid var(--color-border);
    transition: all 0.15s;
  }
  .dialog-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .dialog-btn.cancel {
    background: var(--color-surface-2);
    color: var(--color-text-dim);
  }
  .dialog-btn.cancel:hover:not(:disabled) {
    border-color: var(--color-border-hover);
    color: var(--color-text);
  }
  .dialog-btn.confirm {
    background: #ef4444;
    border-color: #ef4444;
    color: white;
  }
  .dialog-btn.confirm:hover:not(:disabled) {
    background: #dc2626;
    border-color: #dc2626;
  }

  /* ── Responsive ──────────────────────────────────────────────────────────── */
  @media (max-width: 640px) {
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
    .lightbox {
      padding: 3.5rem 1rem 1rem;
    }
  }
</style>
