// Browser-based IndexedDB cache for tiles
class TileCache {
  constructor(dbName = 'watercolormap-tiles') {
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
        if (!db.objectStoreNames.contains('tiles')) {
          db.createObjectStore('tiles', { keyPath: 'key' });
        }
      };
    });
  }

  async get(z, x, y, is2x = false) {
    await this.ready;
    const suffix = is2x ? '@2x' : '';
    const key = `z${z}_x${x}_y${y}${suffix}`;

    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(['tiles'], 'readonly');
      const store = tx.objectStore('tiles');
      const req = store.get(key);
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve(req.result?.blob);
    });
  }

  async set(z, x, y, blob, is2x = false) {
    await this.ready;
    const suffix = is2x ? '@2x' : '';
    const key = `z${z}_x${x}_y${y}${suffix}`;

    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(['tiles'], 'readwrite');
      const store = tx.objectStore('tiles');
      const req = store.put({ key, blob, timestamp: Date.now() });
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve();
    });
  }

  async clear() {
    await this.ready;
    return new Promise((resolve, reject) => {
      const tx = this.db.transaction(['tiles'], 'readwrite');
      const store = tx.objectStore('tiles');
      const req = store.clear();
      req.onerror = () => reject(req.error);
      req.onsuccess = () => resolve();
    });
  }
}

// Demo UI Manager
class WaterColorMapPlayground {
  constructor() {
    this.cache = new TileCache();
    this.map = null;
    this.tileLayer = null;
    this.generatingTiles = new Set();
    this.statusEl = document.getElementById('status');
    this.init();
  }

  async init() {
    // Initialize map
    const hanoverCenter = [52.375, 9.732];
    this.map = L.map('map', {
      center: hanoverCenter,
      zoom: 13,
      minZoom: 10,
      maxZoom: 16,
    });

    // Custom tile layer that uses WASM generation and caching
    this.tileLayer = L.tileLayer('', {
      attribution: '© OpenStreetMap | WaterColorMap WASM Playground',
      detectRetina: true,
      maxZoom: 16,
    });

    this.tileLayer.getTileUrl = ({ x, y, z }) => {
      const dpr = window.devicePixelRatio || 1;
      const suffix = dpr >= 2 ? '@2x' : '';
      const url = `data:${z}/${x}/${y}${suffix}`;
      this.loadTileAsync(z, x, y, dpr >= 2);
      return url;
    };

    this.tileLayer.addTo(this.map);

    // Add controls
    this.setupControls();
    this.updateStatus('Ready (WASM mode)');
  }

  setupControls() {
    const controlDiv = document.getElementById('controls');
    controlDiv.innerHTML = `
      <div>
        <button id="clearCache" class="btn">Clear Cache</button>
        <button id="toggleMode" class="btn">Server Mode</button>
      </div>
      <div style="margin-top: 10px; font-size: 12px;">
        <span id="cacheStatus">Cache: -</span><br>
        <span id="renderStatus">Status: -</span>
      </div>
    `;

    document.getElementById('clearCache').addEventListener('click', async () => {
      await this.cache.clear();
      this.updateStatus('Cache cleared');
      this.map._repaint();
    });

    document.getElementById('toggleMode').addEventListener('click', () => {
      alert('Server mode: Point the tile layer at a running watercolormap serve instance');
    });
  }

  async loadTileAsync(z, x, y, is2x) {
    const key = `${z}/${x}/${y}`;
    if (this.generatingTiles.has(key)) return;

    // Check cache first
    const cached = await this.cache.get(z, x, y, is2x);
    if (cached) {
      this.displayCachedTile(z, x, y, cached, is2x);
      return;
    }

    this.generatingTiles.add(key);
    this.updateStatus(`Generating ${key}...`);

    try {
      const req = {
        zoom: z,
        x: y,
        y: y,
        hidpi: is2x,
        base64: true,
      };

      // Call WASM function (if available)
      if (typeof watercolorGenerateTile !== 'undefined') {
        const result = watercolorGenerateTile(JSON.stringify(req));
        if (result.error) {
          console.error(`Tile error: ${result.error}`);
          this.updateStatus(`Error: ${result.error}`);
          return;
        }
        this.updateStatus(result.info || `Generated ${key}`);
      } else {
        this.updateStatus('WASM module not loaded');
      }
    } catch (err) {
      console.error(`Failed to generate tile ${key}:`, err);
      this.updateStatus(`Error: ${err.message}`);
    } finally {
      this.generatingTiles.delete(key);
    }
  }

  displayCachedTile(z, x, y, blob, is2x) {
    const url = URL.createObjectURL(blob);
    const key = `z${z}_x${x}_y${y}${is2x ? '@2x' : ''}`;
    console.log(`Displaying cached tile: ${key}`);
  }

  updateStatus(msg) {
    if (this.statusEl) {
      this.statusEl.textContent = msg;
    }
    console.log(`[Status] ${msg}`);
  }
}

// Initialize on page load
window.addEventListener('load', () => {
  if (typeof watercolorInit !== 'undefined') {
    watercolorInit();
  }
  window.playground = new WaterColorMapPlayground();
});
