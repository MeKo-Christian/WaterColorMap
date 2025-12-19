// Browser-based IndexedDB cache for tiles
class TileCache {
  constructor(dbName = "watercolormap-tiles") {
    this.dbName = dbName;
    this.db = null;
    this.ready = this.init();
  }

  async init() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open(this.dbName, 1);
      req.onerror = () => reject(req.error);
      req.onsuccess = () => {
        this.db = req.result;
        resolve();
      };
      req.onupgradeneeded = (e) => {
        const db = e.target.result;
        if (!db.objectStoreNames.contains("tiles")) {
          db.createObjectStore("tiles", { keyPath: "key" });
        }
      };
    });
  }

  async get(z, x, y, is2x = false) {
    await this.ready;
    const suffix = is2x ? "@2x" : "";
    const key = `z${z}_x${x}_y${y}${suffix}`;

    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(["tiles"], "readonly");
      const store = tx.objectStore("tiles");
      const req = store.get(key);
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve(req.result?.blob);
    });
  }

  async set(z, x, y, blob, is2x = false) {
    await this.ready;
    const suffix = is2x ? "@2x" : "";
    const key = `z${z}_x${x}_y${y}${suffix}`;

    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(["tiles"], "readwrite");
      const store = tx.objectStore("tiles");
      const req = store.put({ key, blob, timestamp: Date.now() });
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve();
    });
  }

  async clear() {
    await this.ready;
    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(["tiles"], "readwrite");
      const store = tx.objectStore("tiles");
      const req = store.clear();
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve();
    });
  }
}

// Simple semaphore for limiting concurrent operations
class Semaphore {
  constructor(max) {
    this.max = max;
    this.count = 0;
    this.queue = [];
  }

  async acquire() {
    if (this.count < this.max) {
      this.count++;
      return;
    }
    await new Promise((resolve) => this.queue.push(resolve));
    this.count++;
  }

  release() {
    this.count--;
    if (this.queue.length > 0) {
      const next = this.queue.shift();
      next();
    }
  }
}

// Get concurrency from WASM or fallback to navigator.hardwareConcurrency or 4
function getMaxConcurrency() {
  // Try WASM function first
  if (typeof watercolorGetConcurrency === "function") {
    try {
      const cores = watercolorGetConcurrency();
      if (typeof cores === "number" && cores > 0) {
        return cores;
      }
    } catch (e) {
      // fall through
    }
  }
  // Fallback to browser API
  if (navigator.hardwareConcurrency) {
    return navigator.hardwareConcurrency;
  }
  // Default
  return 4;
}

// Demo UI Manager
class WaterColorMapPlayground {
  constructor() {
    this.cache = new TileCache();
    this.map = null;
    this.tileLayer = null;
    this.statusEl = document.getElementById("status");
    this.backendBaseUrl = this.getInitialBackendBaseUrl();
    this.maxConcurrency = getMaxConcurrency();
    this.fetchSemaphore = new Semaphore(this.maxConcurrency);
    this.init();
  }

  getInitialBackendBaseUrl() {
    const params = new URLSearchParams(window.location.search);
    const fromQuery = params.get("backend");
    // Default to localhost for local development.
    return (fromQuery || "http://127.0.0.1:8080").replace(/\/$/, "");
  }

  async init() {
    // Initialize map
    const hanoverCenter = [52.375, 9.732];
    this.map = L.map("map", {
      center: hanoverCenter,
      zoom: 13,
      minZoom: 10,
      maxZoom: 16,
    });

    this.tileLayer = this.createGridLayer();

    this.tileLayer.addTo(this.map);

    // Add controls
    this.setupControls();
    this.updateStatus(
      `Ready. Backend: ${this.backendBaseUrl} (${this.maxConcurrency} CPUs)`
    );
  }

  createGridLayer() {
    const self = this;
    const WaterColorGridLayer = L.GridLayer.extend({
      createTile(coords, done) {
        const img = document.createElement("img");
        img.alt = "";
        img.setAttribute("role", "presentation");

        const dpr = window.devicePixelRatio || 1;
        const is2x = dpr >= 2;
        const z = coords.z;
        const x = coords.x;
        const y = coords.y;

        self
          .loadTileToImg({ z, x, y, is2x, img })
          .then(() => done(null, img))
          .catch((err) => {
            console.warn("tile load failed", err);
            done(err, img);
          });

        return img;
      },
    });

    return new WaterColorGridLayer({
      attribution: "© OpenStreetMap contributors | WaterColorMap Playground",
      tileSize: 256,
      maxZoom: 16,
      minZoom: 10,
      updateWhenIdle: true,
      updateWhenZooming: false,
      keepBuffer: 2,
    });
  }

  makeTileUrl(z, x, y, is2x) {
    const suffix = is2x ? "@2x" : "";

    // If WASM is available, let it compute the canonical filename.
    if (typeof watercolorGenerateTile === "function") {
      try {
        const req = { zoom: z, x, y, hidpi: is2x };
        const res = watercolorGenerateTile(JSON.stringify(req));
        if (res && res.filename) {
          return `${this.backendBaseUrl}/tiles/${res.filename}`;
        }
      } catch (e) {
        // fall through
      }
    }

    // Fallback (no WASM): match the server's flat filename scheme.
    return `${this.backendBaseUrl}/tiles/z${z}_x${x}_y${y}${suffix}.png`;
  }

  async loadTileToImg({ z, x, y, is2x, img }) {
    // Cache-first
    const cached = await this.cache.get(z, x, y, is2x);
    if (cached) {
      const objectUrl = URL.createObjectURL(cached);
      img.onload = () => URL.revokeObjectURL(objectUrl);
      img.onerror = () => URL.revokeObjectURL(objectUrl);
      img.src = objectUrl;
      return;
    }

    // Fetch from backend with concurrency limit
    await this.fetchSemaphore.acquire();
    try {
      const url = this.makeTileUrl(z, x, y, is2x);
      this.updateStatus(
        `Fetching z${z} ${x}/${y}... (${this.maxConcurrency} concurrent)`
      );

      let resp;
      try {
        resp = await fetch(url, { mode: "cors" });
      } catch (err) {
        img.src = this.makePlaceholderDataUrl("Backend unreachable");
        this.updateStatus(`Backend unreachable: ${this.backendBaseUrl}`);
        return;
      }

      if (!resp.ok) {
        img.src = this.makePlaceholderDataUrl(`HTTP ${resp.status}`);
        this.updateStatus(`Tile fetch failed: HTTP ${resp.status}`);
        return;
      }

      const blob = await resp.blob();
      await this.cache.set(z, x, y, blob, is2x);

      const objectUrl = URL.createObjectURL(blob);
      img.onload = () => URL.revokeObjectURL(objectUrl);
      img.onerror = () => URL.revokeObjectURL(objectUrl);
      img.src = objectUrl;
    } finally {
      this.fetchSemaphore.release();
    }
  }

  makePlaceholderDataUrl(message) {
    const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256">
  <rect width="100%" height="100%" fill="#f5f5f5"/>
  <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#777">${String(
    message
  )
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")}</text>
</svg>`;
    return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  }

  setupControls() {
    const controlDiv = document.getElementById("controls");
    controlDiv.innerHTML = `
      <div>
        <button id="clearCache" class="btn">Clear Cache</button>
        <button id="toggleMode" class="btn">Backend URL</button>
      </div>
      <div style="margin-top: 10px; font-size: 12px;">
        <span id="cacheStatus">Cache: -</span><br>
        <span id="renderStatus">Status: -</span>
      </div>
    `;

    document
      .getElementById("clearCache")
      .addEventListener("click", async () => {
        await this.cache.clear();
        this.updateStatus("Cache cleared");
        this.map._repaint();
      });

    document.getElementById("toggleMode").addEventListener("click", () => {
      const next = prompt(
        "Backend base URL (example: http://127.0.0.1:8080).\n\nYou can also set ?backend=... in the URL.",
        this.backendBaseUrl
      );
      if (!next) return;
      this.backendBaseUrl = next.replace(/\/$/, "");
      this.updateStatus(`Backend set: ${this.backendBaseUrl}`);
      this.map._repaint();
    });
  }

  updateStatus(msg) {
    if (this.statusEl) {
      this.statusEl.textContent = msg;
    }
    console.log(`[Status] ${msg}`);
  }
}

// Initialize on page load
window.addEventListener("load", () => {
  window.playground = new WaterColorMapPlayground();
});
