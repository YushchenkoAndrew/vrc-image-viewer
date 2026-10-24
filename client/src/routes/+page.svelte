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
    zoom: number;
    panning: boolean;
    pan: { x: number; y: number };
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
    zoom: 1,
    panning: false,
    pan: { x: 0, y: 0 },
    crop: null,
  };

  // ── Persistence: localStorage + remote ────────────────────────────────────
  const LS_KEY = 'vrsync-edits-v2';

  function loadAllLocal(): Record<string, EditState> {
    try {
      return JSON.parse(localStorage.getItem(LS_KEY) ?? '{}');
    } catch {
      return {};
    }
  }
  function saveAllLocal(edits: Record<string, EditState>) {
    localStorage.setItem(LS_KEY, JSON.stringify(edits));
  }

  // POST images/:id/settings  — fire-and-forget, errors are silent
  function saveRemote(id: string, e: EditState) {
    fetch(`/images/${id}/settings`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ settings: JSON.stringify(e) }),
    }).catch(() => {});
  }

  // GET images/:id/settings → returns { settings: "<json string>" }
  // Returns null on any error so caller falls back to localStorage
  async function loadRemote(id: string): Promise<EditState | null> {
    try {
      const res = await fetch(`/images/${id}/settings`);
      if (!res.ok) return null;
      const json = await res.json();
      return normalize(JSON.parse(json.settings) as EditState);
    } catch {
      return null;
    }
  }

  let allEdits = $state<Record<string, EditState>>({});

  function normalize(raw: Partial<EditState>): EditState {
    return { ...JSON.parse(JSON.stringify(DEFAULT_EDIT)), ...raw };
  }

  function getEdit(src: string): EditState {
    return normalize(allEdits[src]);
  }

  function setEdit(src: string, id: string, e: EditState) {
    allEdits = { ...allEdits, [src]: e };
    saveAllLocal(allEdits);
    saveRemote(id, e);
  }

  function clearEdit(src: string, id: string) {
    const copy = { ...allEdits };
    delete copy[src];
    allEdits = copy;
    saveAllLocal(allEdits);
    saveRemote(id, { ...DEFAULT_EDIT });
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

  // Tracks the edit state as it was when the editor was opened (after remote load).
  // We only POST if the current edit differs from this baseline.
  let editBaseline = $state<EditState | null>(null);

  // Auto-save edits whenever `edit` changes while editor is open.
  // We must read every tracked field BEFORE any early return so Svelte
  // registers them as dependencies regardless of the guard condition.
  let autoSaveTimer: ReturnType<typeof setTimeout>;
  $effect(() => {
    // Destructure all fields so Svelte 5 tracks each one as a dependency
    const snapshot: EditState = {
      rotation: edit.rotation,
      flipH: edit.flipH,
      flipV: edit.flipV,
      brightness: edit.brightness,
      contrast: edit.contrast,
      saturation: edit.saturation,
      hue: edit.hue,
      zoom: edit.zoom,
      panning: false,
      pan: { x: 0, y: 0 },
      crop: edit.crop ? { ...edit.crop } : null,
    };
    const img = editorImage;
    const open = editorOpen;
    const base = editBaseline;
    // Guard AFTER reading all reactive values
    if (!open || !img || !base) return;
    // Only save if something actually changed from the loaded baseline
    if (JSON.stringify(snapshot) === JSON.stringify(base)) return;
    clearTimeout(autoSaveTimer);
    autoSaveTimer = setTimeout(() => {
      setEdit(img.src, img.id, snapshot);
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
    return [
      `translate(${e.pan?.x ?? 0}px, ${e.pan?.y ?? 0}px)`,
      `rotate(${e.rotation}deg)`,
      `scale(${e.zoom * (e.flipH ? -1 : 1)}, ${e.zoom * (e.flipV ? -1 : 1)})`,
    ].join(' ');
  }

  // ── Download with canvas ───────────────────────────────────────────────────
  // Crop coords (x,y,w,h in %) are expressed in the DISPLAYED coordinate space
  // (after CSS rotation/flip). We must transform them back to natural image space.
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

    const NW = image.naturalWidth,
      NH = image.naturalHeight;
    const rot = ed.rotation;

    // Step 1: figure out the natural-space crop rectangle.
    // Crop percentages are in DISPLAYED space (after rotation).
    // Displayed dimensions after rotation:
    const swapped = rot === 90 || rot === 270;
    // dispW/dispH = size of the image as the user sees it
    // (for percentage math, actual px doesn't matter — just which axis is which)
    let cx_n: number, cy_n: number, cw_n: number, ch_n: number;

    if (ed.crop) {
      const { x: px, y: py, w: pw, h: ph } = ed.crop; // 0-100 in display space

      // Convert display % → natural % by inverting the rotation transform.
      // The CSS rotation rotates the image around its center.
      // We need to map display (px,py) → natural (nx,ny).
      //
      // For each rotation angle, the displayed top-left in natural coords:
      //   rot=0:   natural (px, py)
      //   rot=90:  natural (py, 100-px-pw) for origin, natural axes swap
      //   rot=180: natural (100-px-pw, 100-py-ph)
      //   rot=270: natural (100-py-ph, px)
      //
      // Accounting for flipH/flipV is applied AFTER rotation in CSS,
      // so we also invert those.

      let [dx, dy, dw, dh] = [px, py, pw, ph];

      // Invert flipH (applied in display space after rotation)
      if (ed.flipH) {
        dx = 100 - dx - dw;
      }
      // Invert flipV
      if (ed.flipV) {
        dy = 100 - dy - dh;
      }

      // Invert rotation to get natural-space rectangle
      if (rot === 0) {
        cx_n = dx;
        cy_n = dy;
        cw_n = dw;
        ch_n = dh;
      } else if (rot === 90) {
        // display x → natural y (from bottom), display y → natural x
        cx_n = dy;
        cy_n = 100 - dx - dw;
        cw_n = dh;
        ch_n = dw;
      } else if (rot === 180) {
        cx_n = 100 - dx - dw;
        cy_n = 100 - dy - dh;
        cw_n = dw;
        ch_n = dh;
      } else {
        // 270
        cx_n = 100 - dy - dh;
        cy_n = dx;
        cw_n = dh;
        ch_n = dw;
      }

      // Convert % to pixels in natural space
      cx_n = (cx_n / 100) * NW;
      cy_n = (cy_n / 100) * NH;
      cw_n = (cw_n / 100) * NW;
      ch_n = (ch_n / 100) * NH;
    } else {
      cx_n = 0;
      cy_n = 0;
      cw_n = NW;
      ch_n = NH;
    }

    // Step 2: output canvas size = crop region after rotation
    const outW = swapped ? ch_n : cw_n;
    const outH = swapped ? cw_n : ch_n;

    const canvas = document.createElement('canvas');
    canvas.width = Math.round(outW);
    canvas.height = Math.round(outH);
    const ctx = canvas.getContext('2d')!;
    ctx.filter = editToFilter(ed);
    ctx.save();
    ctx.translate(outW / 2, outH / 2);
    ctx.rotate((rot * Math.PI) / 180);
    if (ed.flipH) ctx.scale(-1, 1);
    if (ed.flipV) ctx.scale(1, -1);
    // Draw the natural crop region, centered
    ctx.drawImage(
      image,
      cx_n,
      cy_n,
      cw_n,
      ch_n,
      -cw_n / 2,
      -ch_n / 2,
      cw_n,
      ch_n,
    );
    ctx.restore();

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
    allEdits = loadAllLocal();
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
  async function openEditor(img: ImageFile) {
    editorImage = img;
    edit = getEdit(img.src);
    editBaseline = null;

    cropMode = false;
    cropDraft = null;
    cropStart = null;
    editorOpen = true;
    document.body.style.overflow = 'hidden';

    const remote = await loadRemote(img.id);

    // Ignore stale responses if another image was opened.
    if (editorImage?.src !== img.src) return;
    if (remote) edit = normalize(remote);

    // Persist the complete settings object locally and remotely.
    allEdits = { ...allEdits, [img.src]: normalize(edit) };
    saveAllLocal(allEdits);
    saveRemote(img.id, edit);

    // Set the baseline after loading.
    editBaseline = normalize(edit);
  }
  function closeEditor() {
    editorOpen = false;
    editorImage = null;
    editBaseline = null;
    cropMode = false;
    cropDraft = null;
    document.body.style.overflow = '';
  }

  // ── Transform ──────────────────────────────────────────────────────────────
  function rotate(deg: number) { edit = { ...edit, rotation: ((((edit.rotation || 0) + deg) % 360) + 360) % 360 }; } // prettier-ignore
  function toggleFlipH() { edit = { ...edit, flipH: !edit.flipH }; } // prettier-ignore
  function toggleFlipV() { edit = { ...edit, flipV: !edit.flipV }; } // prettier-ignore

  let panStart = { x: 0, y: 0 };
  let pointerStart = { x: 0, y: 0 };

  function onPanUp() { edit.panning = false; } // prettier-ignore
  function onPanDown(e: PointerEvent) {
    if (cropMode || edit.crop || e.button !== 0) return;

    e.preventDefault();
    edit.panning = true;

    pointerStart = { x: e.clientX, y: e.clientY };
    panStart = { x: edit.pan?.x || 0, y: edit.pan?.y || 0 };

    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }

  function onPanMove(e: PointerEvent) {
    if (!(e.currentTarget as HTMLElement).hasPointerCapture(e.pointerId)) return; // prettier-ignore
    edit = { ...edit, pan: { x: panStart.x + (e.clientX - pointerStart.x) * (edit.zoom || 1), y: panStart.y + (e.clientY - pointerStart.y) * (edit.zoom || 1) } }; // prettier-ignore
  }

  // ── Crop ───────────────────────────────────────────────────────────────────
  let imgEl: HTMLImageElement;
  let imgRect = $state<{ left: number; top: number; width: number; height: number; } | null>(null); // prettier-ignore

  function updateImgRect() {
    if (!imgEl) return;
    const r = imgEl.getBoundingClientRect();
    imgRect = { left: r.left, top: r.top, width: r.width, height: r.height };
  }

  function enterCropMode() { cropMode = true; cropDraft = cropStart = null; requestAnimationFrame(() => requestAnimationFrame(updateImgRect)); } // prettier-ignore
  function resetCrop() { edit = { ...edit, crop: null }; } // prettier-ignore
  function cancelCrop() { cropMode = false; cropDraft = cropStart = null; cropAction = 'none'; } // prettier-ignore
  function applyCrop() {
    if (cropDraft) edit = { ...edit, crop: { ...cropDraft } }; // prettier-ignore

    cropMode = false;
    cropDraft = null;
    cropAction = 'none';
  }

  function imgPct(cx: number, cy: number) {
    const r = imgRect!;
    return {
      x: Math.max(0, Math.min(100, ((cx - r.left) / r.width) * 100)),
      y: Math.max(0, Math.min(100, ((cy - r.top) / r.height) * 100)),
    };
  }

  // ── Crop interaction state machine ─────────────────────────────────────────
  type CropAction =
    | 'none'
    | 'draw'
    | 'move'
    | 'resize-tl'
    | 'resize-t'
    | 'resize-tr'
    | 'resize-r'
    | 'resize-br'
    | 'resize-b'
    | 'resize-bl'
    | 'resize-l';

  let cropAction = $state<CropAction>('none');
  // Snapshot of cropDraft at pointer-down, plus pointer-down position
  let cropActionStart = $state<{
    px: number;
    py: number;
    rx: number;
    ry: number;
    rw: number;
    rh: number;
  } | null>(null);

  // Which cursor to show based on hover handle
  let cropCursor = $state('crosshair');

  const HANDLE_HIT = 8; // % of rect dimension — hit zone for handles

  function hitTest(
    p: { x: number; y: number },
    d: typeof cropDraft,
  ): CropAction {
    if (!d || d.w < 0.5 || d.h < 0.5) return 'draw';
    const { x, y, w, h } = d;
    const ex = x + w,
      ey = y + h;
    // hit threshold in percentage points (fixed 1.5% of image, min 6px feel)
    const HX = Math.min(HANDLE_HIT, w * 0.35);
    const HY = Math.min(HANDLE_HIT, h * 0.35);

    const inX = p.x >= x && p.x <= ex;
    const inY = p.y >= y && p.y <= ey;
    const onL = inY && p.x >= x - HX && p.x <= x + HX;
    const onR = inY && p.x >= ex - HX && p.x <= ex + HX;
    const onT = inX && p.y >= y - HY && p.y <= y + HY;
    const onB = inX && p.y >= ey - HY && p.y <= ey + HY;

    if (onL && onT) return 'resize-tl';
    if (onR && onT) return 'resize-tr';
    if (onR && onB) return 'resize-br';
    if (onL && onB) return 'resize-bl';
    if (onT) return 'resize-t';
    if (onB) return 'resize-b';
    if (onL) return 'resize-l';
    if (onR) return 'resize-r';
    if (inX && inY) return 'move';
    return 'draw';
  }

  const ACTION_CURSOR: Record<CropAction, string> = {
    none: 'crosshair',
    draw: 'crosshair',
    move: 'move',
    'resize-tl': 'nwse-resize',
    'resize-br': 'nwse-resize',
    'resize-tr': 'nesw-resize',
    'resize-bl': 'nesw-resize',
    'resize-t': 'ns-resize',
    'resize-b': 'ns-resize',
    'resize-l': 'ew-resize',
    'resize-r': 'ew-resize',
  };

  function onCropMouseMove(e: PointerEvent) {
    if (cropAction !== 'none') {
      onCropDrag(e);
      return;
    }
    if (!cropDraft) {
      cropCursor = 'crosshair';
      return;
    }
    const p = imgPct(e.clientX, e.clientY);
    const action = hitTest(p, cropDraft);
    cropCursor = ACTION_CURSOR[action];
  }

  function onCropDown(e: PointerEvent) {
    e.preventDefault();
    e.stopPropagation();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    const p = imgPct(e.clientX, e.clientY);
    const action = cropDraft ? hitTest(p, cropDraft) : 'draw';
    cropAction = action;
    if (action === 'draw') {
      cropStart = p;
      cropDraft = { x: p.x, y: p.y, w: 0, h: 0 };
      cropActionStart = null;
    } else {
      cropActionStart = {
        px: p.x,
        py: p.y,
        rx: cropDraft!.x,
        ry: cropDraft!.y,
        rw: cropDraft!.w,
        rh: cropDraft!.h,
      };
    }
  }

  function onCropDrag(e: PointerEvent) {
    const p = imgPct(e.clientX, e.clientY);

    if (cropAction === 'draw' && cropStart) {
      cropDraft = {
        x: Math.min(cropStart.x, p.x),
        y: Math.min(cropStart.y, p.y),
        w: Math.abs(p.x - cropStart.x),
        h: Math.abs(p.y - cropStart.y),
      };
      return;
    }

    if (!cropActionStart || !cropDraft) return;
    const { px, py, rx, ry, rw, rh } = cropActionStart;
    const dx = p.x - px,
      dy = p.y - py;

    const clamp = (v: number, lo: number, hi: number) =>
      Math.max(lo, Math.min(hi, v));

    if (cropAction === 'move') {
      cropDraft = {
        x: clamp(rx + dx, 0, 100 - rw),
        y: clamp(ry + dy, 0, 100 - rh),
        w: rw,
        h: rh,
      };
      return;
    }

    let { x, y, w, h } = { x: rx, y: ry, w: rw, h: rh };
    const MIN = 2; // minimum crop size %

    if (cropAction.includes('l')) {
      const nx = clamp(rx + dx, 0, rx + rw - MIN);
      x = nx;
      w = rx + rw - nx;
    }
    if (cropAction.includes('r')) {
      w = clamp(rw + dx, MIN, 100 - rx);
    }
    if (cropAction.includes('t')) {
      const ny = clamp(ry + dy, 0, ry + rh - MIN);
      y = ny;
      h = ry + rh - ny;
    }
    if (cropAction.includes('b')) {
      h = clamp(rh + dy, MIN, 100 - ry);
    }
    cropDraft = { x, y, w, h };
  }

  function onCropUp(e: PointerEvent) {
    e.stopPropagation();
    cropAction = 'none';
    cropStart = null;
    cropActionStart = null;
  }

  $effect(() => {
    void edit.rotation;
    if (cropMode) requestAnimationFrame(updateImgRect);
  });

  // ── Delete ─────────────────────────────────────────────────────────────────
  async function confirmDelete() {
    if (!deleteTarget) return;
    deleting = true;
    try {
      const res = await fetch(`/images/${deleteTarget.id}`, {
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

  // ── Canvas crop preview ────────────────────────────────────────────────────
  // When a crop is active (and not in crop-draw mode), we render the result
  // onto a canvas so the display matches the download exactly.
  let previewCanvas: HTMLCanvasElement;
  let previewUrl = $state<string | null>(null);

  async function renderCropPreview(img: ImageFile, ed: EditState) {
    if (!ed.crop) {
      previewUrl = null;
      return;
    }

    const image = new Image();
    image.crossOrigin = 'anonymous';
    image.src = img.src;
    await image.decode();

    const { naturalWidth: NW, naturalHeight: NH } = image;
    const rot = (((ed.rotation || 0) % 360) + 360) % 360;
    const swapped = rot === 90 || rot === 270;

    // Dimensions after rotation
    const RW = swapped ? NH : NW;
    const RH = swapped ? NW : NH;

    // Crop values are percentages of the rotated image
    const { x, y, w, h } = ed.crop;

    const sx = (x / 100) * RW;
    const sy = (y / 100) * RH;
    const sw = (w / 100) * RW;
    const sh = (h / 100) * RH;

    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(sw));
    canvas.height = Math.max(1, Math.round(sh));

    const ctx = canvas.getContext('2d')!;

    // Map the original image into the rotated coordinate system.
    ctx.translate(RW / 2, RH / 2);
    ctx.rotate((rot * Math.PI) / 180);
    ctx.scale(ed.flipH ? -1 : 1, ed.flipV ? -1 : 1);

    // Draw the full image centered around the origin.
    ctx.drawImage(image, -NW / 2, -NH / 2);

    // Crop using a second canvas because the first canvas is the output.
    const rotatedCanvas = document.createElement('canvas');
    rotatedCanvas.width = RW;
    rotatedCanvas.height = RH;

    const rctx = rotatedCanvas.getContext('2d')!;
    rctx.translate(RW / 2, RH / 2);
    rctx.rotate((rot * Math.PI) / 180);
    rctx.scale(ed.flipH ? -1 : 1, ed.flipV ? -1 : 1);
    rctx.drawImage(image, -NW / 2, -NH / 2);

    const cropCanvas = document.createElement('canvas');
    cropCanvas.width = Math.max(1, Math.round(sw));
    cropCanvas.height = Math.max(1, Math.round(sh));

    cropCanvas
      .getContext('2d')!
      .drawImage(
        rotatedCanvas,
        sx, sy, sw, sh,
        0, 0, cropCanvas.width, cropCanvas.height,
      ); // prettier-ignore

    previewUrl = cropCanvas.toDataURL('image/jpeg', 0.92);
  }

  // Re-render preview whenever edit changes and crop is set
  $effect(() => {
    const snap = { ...edit };
    const img = editorImage;
    if (!img || !snap.crop || cropMode) {
      previewUrl = null;
      return;
    }
    renderCropPreview(img, snap);
  });
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
      class="query-textarea"></textarea>
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
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
  <div class="editor-backdrop" onclick={closeEditor}>
    <!-- ── Canvas area (center, always fills remaining space) ── -->
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
    <div class="editor-canvas" class:crop-mode={cropMode}>
      <!-- Editor topbar -->
      <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
      <div class="editor-topbar" onclick={(e) => e.stopPropagation()}>
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
          class:can-pan={!cropMode && !edit.crop}
          onpointerdown={onPanDown}
          onpointermove={onPanMove}
          onpointerup={onPanUp}
          onclick={(e) => {
            if (!imgEl) return;

            const rect = imgEl.getBoundingClientRect();
            const { clientX: x, clientY: y } = e;

            if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) closeEditor();
            else e.stopPropagation(); // prettier-ignore
          }}
        >
          {#if previewUrl && !cropMode}
            <!-- Canvas-rendered crop preview: exact same output as download -->
            <img
              src={previewUrl}
              alt="Crop preview"
              class="editor-img preview-img"
              style="transform:scale({edit.zoom}, {edit.zoom}) !important;"
            />
          {:else}
            <!-- Live image with CSS rotation/flip/filter -->
            <img
              bind:this={imgEl}
              src={editorImage.src}
              alt={editorImage.name}
              class="editor-img"
              class:panning={edit.panning}
              class:rotated={edit.rotation === 90 || edit.rotation === 270}
              style="filter:{editorFilter};transform:{editorTransform};"
            />
          {/if}

          <!-- Crop overlay: fixed-positioned over imgEl's visual bounding rect -->
          {#if cropMode && imgRect}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
            <div
              class="crop-overlay"
              style="position:fixed;left:{imgRect.left}px;top:{imgRect.top}px;width:{imgRect.width}px;height:{imgRect.height}px;z-index:200;cursor:{cropCursor};"
              onpointerdown={onCropDown}
              onpointermove={onCropMouseMove}
              onpointerup={onCropUp}
            >
              {#if cropDraft}
                <!-- Dark shade outside crop -->
                <div class="crop-shade"></div>
                <!-- Crop rectangle -->
                <div
                  class="crop-rect"
                  style="left:{cropDraft.x}%;top:{cropDraft.y}%;width:{cropDraft.w}%;height:{cropDraft.h}%"
                >
                  <!-- Rule of thirds -->
                  <div class="rof h1"></div>
                  <div class="rof h2"></div>
                  <div class="rof v1"></div>
                  <div class="rof v2"></div>
                  <!-- 8 resize handles -->
                  <div class="ch ch-tl" style="cursor:nwse-resize"></div>
                  <div class="ch ch-t" style="cursor:ns-resize"></div>
                  <div class="ch ch-tr" style="cursor:nesw-resize"></div>
                  <div class="ch ch-r" style="cursor:ew-resize"></div>
                  <div class="ch ch-br" style="cursor:nwse-resize"></div>
                  <div class="ch ch-b" style="cursor:ns-resize"></div>
                  <div class="ch ch-bl" style="cursor:nesw-resize"></div>
                  <div class="ch ch-l" style="cursor:ew-resize"></div>
                  <!-- Move cursor interior (above shade, below handles) -->
                  <div class="crop-interior" style="cursor:move"></div>
                </div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- Crop mode floating bar — stopPropagation prevents backdrop close -->
        {#if cropMode}
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
          <div class="crop-bar" onclick={(e) => e.stopPropagation()}>
            <span class="crop-hint">Drag to select · Esc to cancel</span>
            <button
              class="crop-btn apply"
              onclick={(e) => {
                e.stopPropagation();
                applyCrop();
              }}
              disabled={!cropDraft || cropDraft.w < 0.5}>Apply</button
            >
            <button
              class="crop-btn cancel"
              onclick={(e) => {
                e.stopPropagation();
                cancelCrop();
              }}>Cancel</button
            >
          </div>
        {/if}

        <!-- Active crop indicator + reset -->
        {#if edit.crop && !cropMode}
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
          <div class="crop-active-bar" onclick={(e) => e.stopPropagation()}>
            <span class="crop-active-label">
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
                <path d="M6 2v14a2 2 0 0 0 2 2h14" /><path
                  d="M18 22V8a2 2 0 0 0-2-2H2"
                />
              </svg>
              Cropped {edit.crop.w.toFixed(0)}×{edit.crop.h.toFixed(0)}%
            </span>
            <button
              class="crop-reset-btn"
              onclick={(e) => {
                e.stopPropagation();
                resetCrop();
              }}>✕ Reset crop</button
            >
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
      <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
      <aside class="tool-panel" onclick={(e) => e.stopPropagation()}>
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
                {cropMode
                  ? 'Drawing crop…'
                  : edit.crop
                    ? 'Redraw crop'
                    : 'Draw crop'}
              </button>
              {#if edit.crop}
                <button class="tb tb-danger" onclick={() => resetCrop()}
                  >✕ Reset</button
                >
              {/if}
            </div>
            {#if edit.crop}
              <div class="tp-chip">
                Active · {edit.crop.w.toFixed(0)}×{edit.crop.h.toFixed(0)}%
              </div>
            {/if}
          </div>

          <!-- Adjustments -->
          <div class="tp-section">
            <div class="tp-section-label">Adjustments</div>
            {#each [{ key: 'brightness', label: 'Brightness', min: 0, max: 200, def: 100 }, { key: 'contrast', label: 'Contrast', min: 0, max: 200, def: 100 }, { key: 'saturation', label: 'Saturation', min: 0, max: 200, def: 100 }, { key: 'hue', label: 'Hue', min: 0, max: 360, def: 0 }, { key: 'zoom', label: 'Zoom', min: 0.25, max: 5, def: 1, step: '0.05' }] as adj}
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
                    step={adj.step || 1}
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
                clearEdit(editorImage!.src, editorImage!.id);
                edit = { ...DEFAULT_EDIT };
                editBaseline = { ...DEFAULT_EDIT };
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
    touch-action: none;
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .crop-zone.can-pan {
    cursor: grab;
  }
  .editor-img {
    display: block;
    max-width: 100cqw;
    max-height: 100cqh;
    width: auto;
    height: auto;
    object-fit: contain;
    border-radius: 2px;
    transition:
      filter 0.2s ease,
      transform 0.25s ease;
    transform-origin: center center;
  }
  .editor-img.panning {
    transition: none;
  }
  .editor-img.rotated {
    max-width: 100cqh;
    max-height: 100cqw;
  }
  /* Preview canvas result — no rotation needed, already baked in */
  .editor-img.preview-img {
    max-width: 100cqw;
    max-height: 100cqh;
    transform: none !important;
    filter: none !important;
  }

  /* Dark semi-transparent overlay outside the draft crop rect */
  .crop-shade {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    pointer-events: none;
    z-index: 1;
  }
  /* The crop-rect sits above the shade, punching through it visually */
  .crop-rect {
    position: absolute;
    border: 1.5px solid rgba(255, 255, 255, 0.9);
    background: transparent;
    pointer-events: none;
    box-shadow: 0 0 0 9999px rgba(0, 0, 0, 0.55);
    z-index: 2;
  }
  .cc {
    position: absolute;
    width: 10px;
    height: 10px;
    border-color: #fff;
    border-style: solid;
  }
  .cc.tl {
    top: -1px;
    left: -1px;
    border-width: 2px 0 0 2px;
  }
  .cc.tr {
    top: -1px;
    right: -1px;
    border-width: 2px 2px 0 0;
  }
  .cc.bl {
    bottom: -1px;
    left: -1px;
    border-width: 0 0 2px 2px;
  }
  .cc.br {
    bottom: -1px;
    right: -1px;
    border-width: 0 2px 2px 0;
  }

  /* 8 resize handles */
  .ch {
    position: absolute;
    background: #fff;
    border: 1.5px solid rgba(0, 0, 0, 0.4);
    border-radius: 2px;
    z-index: 3;
  }
  /* Corners: 10×10 */
  .ch-tl,
  .ch-tr,
  .ch-bl,
  .ch-br {
    width: 10px;
    height: 10px;
  }
  .ch-tl {
    top: -5px;
    left: -5px;
  }
  .ch-tr {
    top: -5px;
    right: -5px;
  }
  .ch-bl {
    bottom: -5px;
    left: -5px;
  }
  .ch-br {
    bottom: -5px;
    right: -5px;
  }
  /* Edges: 8×24, centered */
  .ch-t,
  .ch-b {
    width: 24px;
    height: 8px;
    left: 50%;
    transform: translateX(-50%);
  }
  .ch-l,
  .ch-r {
    width: 8px;
    height: 24px;
    top: 50%;
    transform: translateY(-50%);
  }
  .ch-t {
    top: -4px;
  }
  .ch-b {
    bottom: -4px;
  }
  .ch-l {
    left: -4px;
  }
  .ch-r {
    right: -4px;
  }

  /* Transparent interior for move — sits inside the rect above the shade */
  .crop-interior {
    position: absolute;
    inset: 0;
    z-index: 2;
  }

  /* Rule of thirds */
  .rof {
    position: absolute;
    background: rgba(255, 255, 255, 0.2);
    pointer-events: none;
  }
  .rof.h1 {
    left: 0;
    right: 0;
    top: 33.33%;
    height: 1px;
  }
  .rof.h2 {
    left: 0;
    right: 0;
    top: 66.66%;
    height: 1px;
  }
  .rof.v1 {
    top: 0;
    bottom: 0;
    left: 33.33%;
    width: 1px;
  }
  .rof.v2 {
    top: 0;
    bottom: 0;
    left: 66.66%;
    width: 1px;
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
    z-index: 300;
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

  /* Active crop indicator bar at bottom of stage */
  .crop-active-bar {
    position: absolute;
    bottom: 1rem;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 0.625rem;
    background: color-mix(in srgb, var(--color-surface) 92%, transparent);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: 0.35rem 0.75rem;
    backdrop-filter: blur(8px);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    z-index: 10;
    pointer-events: auto;
  }
  .crop-active-label {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: var(--color-accent);
  }
  .crop-reset-btn {
    background: none;
    border: 1px solid color-mix(in srgb, #ef4444 35%, transparent);
    border-radius: 5px;
    padding: 0.15rem 0.5rem;
    font-size: 0.6875rem;
    font-family: var(--font-mono);
    color: #f87171;
    cursor: pointer;
    transition: all 0.15s;
  }
  .crop-reset-btn:hover {
    background: color-mix(in srgb, #ef4444 12%, transparent);
    border-color: #ef4444;
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
    padding: 0.35rem 0;
    overflow: visible;
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
    margin-top: 1px;
    z-index: 0;
  }
  .adj-slider {
    -webkit-appearance: none;
    appearance: none;
    position: relative;
    z-index: 1;
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
    /* Critical: allow thumb to visually overflow the track height */
    overflow: visible;
  }
  .adj-slider:focus {
    outline: none;
  }
  .adj-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    position: relative;
    z-index: 2;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #ffffff;
    border: 2.5px solid var(--color-accent);
    cursor: pointer;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
    transition:
      box-shadow 0.15s,
      transform 0.1s;
    /* Ensure thumb renders above track */
    margin-top: -6.5px;
  }
  .adj-slider:hover::-webkit-slider-thumb {
    box-shadow:
      0 2px 6px rgba(0, 0, 0, 0.5),
      0 0 0 5px color-mix(in srgb, var(--color-accent) 22%, transparent);
  }
  .adj-slider:active::-webkit-slider-thumb {
    transform: scale(1.12);
    box-shadow:
      0 2px 6px rgba(0, 0, 0, 0.5),
      0 0 0 8px color-mix(in srgb, var(--color-accent) 14%, transparent);
  }
  .adj-slider::-webkit-slider-runnable-track {
    height: 5px;
    border-radius: 3px;
  }
  .adj-slider::-moz-range-thumb {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #ffffff;
    border: 2.5px solid var(--color-accent);
    cursor: pointer;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
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
