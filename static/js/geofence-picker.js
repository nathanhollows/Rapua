// Admin map picker for the geofence block. Writes the hidden "area" field in the
// shape blocks/geometry.go expects: a circle as centre plus radius in metres, or
// a GeoJSON Polygon with a closed ring.
(function () {
	const CIRCLE_STEPS = 64;
	const EARTH_METRES = 6371000;
	const METRES_PER_DEGREE = EARTH_METRES * Math.PI / 180;
	const DEFAULT_RADIUS = 40;
	// Mirrors blocks.DefaultMaxAccuracy: the buffer ring needs a number even
	// when the author has left the field blank.
	const DEFAULT_MAX_ACCURACY = 50;
	const FALLBACK = { lng: 170.50283, lat: -45.87881, zoom: 13 };

	window.__geofencePickers = window.__geofencePickers || {};

	function styleForScheme() {
		const dark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
		return dark
			? "mapbox://styles/nathanhollows/cl9w3nxff002m14sy9fco4vnr"
			: "mapbox://styles/nathanhollows/clszboe2y005i01oid8ca37jm";
	}

	// A circle is stored as a centre and a radius, so the ring drawn here is only a
	// rendering of it and is never what gets saved.
	function circleRing(center, radiusMetres) {
		const latRadians = center.lat * Math.PI / 180;
		const dLat = (radiusMetres / EARTH_METRES) * 180 / Math.PI;
		const dLng = dLat / Math.cos(latRadians);

		const ring = [];
		for (let i = 0; i <= CIRCLE_STEPS; i++) {
			const angle = (i / CIRCLE_STEPS) * 2 * Math.PI;
			ring.push([
				center.lng + dLng * Math.cos(angle),
				center.lat + dLat * Math.sin(angle),
			]);
		}
		return ring;
	}

	// Geometry.Validate accepts any closed ring, including one that crosses itself,
	// where "inside" stops meaning anything. Caught here rather than saved.
	function selfIntersects(ring) {
		const edges = ring.slice(0, -1);
		for (let i = 0; i < edges.length; i++) {
			const a1 = edges[i], a2 = edges[(i + 1) % edges.length];
			for (let j = i + 1; j < edges.length; j++) {
				if (j === i || (j + 1) % edges.length === i || j === (i + 1) % edges.length) continue;
				const b1 = edges[j], b2 = edges[(j + 1) % edges.length];
				if (segmentsCross(a1, a2, b1, b2)) return true;
			}
		}
		return false;
	}

	function segmentsCross(p1, p2, p3, p4) {
		const d1 = cross(p3, p4, p1), d2 = cross(p3, p4, p2);
		const d3 = cross(p1, p2, p3), d4 = cross(p1, p2, p4);
		return ((d1 > 0) !== (d2 > 0)) && ((d3 > 0) !== (d4 > 0));
	}

	function cross(a, b, p) {
		return (b[0] - a[0]) * (p[1] - a[1]) - (b[1] - a[1]) * (p[0] - a[0]);
	}

	function ringAreaMetres(ring) {
		let total = 0;
		const mPerDegLat = 111320;
		for (let i = 0; i < ring.length - 1; i++) {
			const [x1, y1] = ring[i], [x2, y2] = ring[i + 1];
			const scale = Math.cos((y1 + y2) / 2 * Math.PI / 180) * mPerDegLat;
			total += (x1 * scale) * (y2 * mPerDegLat) - (x2 * scale) * (y1 * mPerDegLat);
		}
		return Math.abs(total / 2);
	}

	function humanArea(metres2) {
		if (!metres2) return "";
		return metres2 >= 10000
			? (metres2 / 10000).toFixed(2) + " ha"
			: Math.round(metres2) + " m²";
	}

	// A larger circle around the same centre is exact; how far a real GPS fix
	// might land outside the drawn area, at worst, is the radius plus the
	// accuracy limit.
	function circleBufferRing(center, radius, accuracy) {
		return circleRing(center, radius + accuracy);
	}

	// Offsets every edge outward by `accuracy` metres and re-intersects
	// consecutive edges, the standard way to inflate a polygon. This is a
	// rendering aid only, so a self-crossing result on a sharp concave corner
	// is acceptable; nothing downstream reads it.
	function offsetRingMetres(ring, accuracy) {
		if (!ring || ring.length < 4 || accuracy <= 0) return null;

		const points = ring.slice(0, -1);
		const lat0 = points.reduce(function (sum, p) { return sum + p[1]; }, 0) / points.length;
		const lngScale = Math.cos(lat0 * Math.PI / 180) * METRES_PER_DEGREE;
		const latScale = METRES_PER_DEGREE;
		const toMetres = function (p) { return [p[0] * lngScale, p[1] * latScale]; };
		const toLngLat = function (p) { return [p[0] / lngScale, p[1] / latScale]; };

		const local = points.map(toMetres);
		const n = local.length;
		// Mapbox Draw does not normalise winding to the drawn direction, so the
		// outward side is worked out from the ring's own signed area rather than
		// assumed.
		const sign = ringSignedArea(local) >= 0 ? 1 : -1;

		const edges = [];
		for (let i = 0; i < n; i++) {
			const a = local[i], b = local[(i + 1) % n];
			const dx = b[0] - a[0], dy = b[1] - a[1];
			const len = Math.hypot(dx, dy) || 1;
			const nx = (dy / len) * sign, ny = -(dx / len) * sign;
			edges.push({ a: [a[0] + nx * accuracy, a[1] + ny * accuracy], b: [b[0] + nx * accuracy, b[1] + ny * accuracy] });
		}

		const out = [];
		for (let i = 0; i < n; i++) {
			const prev = edges[(i - 1 + n) % n];
			const curr = edges[i];
			out.push(toLngLat(lineIntersection(prev, curr) || curr.a));
		}
		out.push(out[0]);
		return out;
	}

	function ringSignedArea(local) {
		let area = 0;
		for (let i = 0; i < local.length; i++) {
			const a = local[i], b = local[(i + 1) % local.length];
			area += a[0] * b[1] - b[0] * a[1];
		}
		return area;
	}

	function lineIntersection(edge1, edge2) {
		const x1 = edge1.a[0], y1 = edge1.a[1], x2 = edge1.b[0], y2 = edge1.b[1];
		const x3 = edge2.a[0], y3 = edge2.a[1], x4 = edge2.b[0], y4 = edge2.b[1];
		const denom = (x1 - x2) * (y3 - y4) - (y1 - y2) * (x3 - x4);
		if (Math.abs(denom) < 1e-9) return null;
		const t = ((x1 - x3) * (y3 - y4) - (y1 - y3) * (x3 - x4)) / denom;
		return [x1 + t * (x2 - x1), y1 + t * (y2 - y1)];
	}

	function accuracyValue(picker) {
		const raw = picker.accuracyInput && picker.accuracyInput.value;
		const parsed = raw ? parseFloat(raw) : NaN;
		return Number.isFinite(parsed) && parsed > 0 ? parsed : DEFAULT_MAX_ACCURACY;
	}

	// A shape drawn back on itself has no inside to test, so it is coloured red
	// rather than deleted: the crossing can be dragged straight without losing
	// the rest of the shape. "user_" is how Mapbox Draw exposes properties it
	// doesn't own to its own style expressions.
	function drawStyles() {
		const colorFor = function (valid) {
			return ["case", ["==", ["get", "user_invalid"], true], "#ef4444", valid];
		};
		return [
			{
				id: "gl-draw-polygon-fill", type: "fill",
				filter: ["==", "$type", "Polygon"],
				paint: { "fill-color": colorFor("#22c55e"), "fill-opacity": 0.18 },
			},
			{
				id: "gl-draw-polygon-stroke", type: "line",
				filter: ["==", "$type", "Polygon"],
				layout: { "line-cap": "round", "line-join": "round" },
				paint: { "line-color": colorFor("#22c55e"), "line-width": 2 },
			},
			{
				id: "gl-draw-polygon-midpoint", type: "circle",
				filter: ["all", ["==", "$type", "Point"], ["==", "meta", "midpoint"]],
				paint: { "circle-radius": 3, "circle-color": colorFor("#22c55e") },
			},
			{
				id: "gl-draw-polygon-vertex-halo", type: "circle",
				filter: ["all", ["==", "$type", "Point"], ["==", "meta", "vertex"]],
				paint: { "circle-radius": 6, "circle-color": "#fff" },
			},
			{
				id: "gl-draw-polygon-vertex", type: "circle",
				filter: ["all", ["==", "$type", "Point"], ["==", "meta", "vertex"]],
				paint: { "circle-radius": 4, "circle-color": colorFor("#22c55e") },
			},
		];
	}

	function init(el) {
		el.classList.remove("geofence-map-uninit");

		const previous = window.__geofencePickers[el.id];
		if (previous) { previous.remove(); delete window.__geofencePickers[el.id]; }

		const key = document.getElementById("mapbox_key");
		if (!key || typeof mapboxgl === "undefined") return;
		mapboxgl.accessToken = key.dataset.key;

		const root = el.closest("[data-geofence-picker]");
		const form = document.getElementById(el.dataset.formId);
		const input = form && form.querySelector('input[name="area"]');
		if (!root || !input) return;

		const picker = {
			el, root, form, input,
			tool: "pin",
			radius: DEFAULT_RADIUS,
			ring: null,
			area: parseArea(input.value),
		};

		picker.accuracyInput = form.querySelector('input[name="max_accuracy"]');

		if (picker.area && picker.area.type === "circle") {
			picker.radius = picker.area.radius || DEFAULT_RADIUS;
		} else if (picker.area && picker.area.type === "Polygon") {
			picker.tool = "shape";
			picker.ring = picker.area.coordinates[0];
		}

		const start = startingView(picker);
		picker.map = new mapboxgl.Map({
			container: el.id,
			style: styleForScheme(),
			center: [start.lng, start.lat],
			zoom: start.zoom,
		});
		window.__geofencePickers[el.id] = picker.map;

		picker.map.addControl(new mapboxgl.NavigationControl({ showCompass: false }), "top-right");
		picker.map.on("load", function () { onMapReady(picker); });
	}

	function parseArea(raw) {
		if (!raw || !raw.trim()) return null;
		try {
			const area = JSON.parse(raw);
			return area && area.type ? area : null;
		} catch (err) {
			return null;
		}
	}

	function startingView(picker) {
		const area = picker.area;
		if (area && area.type === "circle" && area.center) {
			return { lng: area.center[0], lat: area.center[1], zoom: zoomForRadius(picker.radius) };
		}
		if (area && area.type === "Polygon" && area.coordinates && area.coordinates[0]) {
			const ring = area.coordinates[0];
			let west = ring[0][0], east = ring[0][0], south = ring[0][1], north = ring[0][1];
			ring.forEach(function (p) {
				west = Math.min(west, p[0]); east = Math.max(east, p[0]);
				south = Math.min(south, p[1]); north = Math.max(north, p[1]);
			});
			return { lng: (west + east) / 2, lat: (south + north) / 2, zoom: 15, bounds: [[west, south], [east, north]] };
		}
		return FALLBACK;
	}

	function zoomForRadius(radius) {
		if (radius <= 25) return 18;
		if (radius <= 75) return 17;
		if (radius <= 200) return 16;
		if (radius <= 600) return 15;
		return 14;
	}

	// Our own sources/layers, unlike Draw's, are not restored automatically when
	// the satellite switcher below calls setStyle: a style swap replaces the
	// whole style, so anything added at runtime has to be re-added afterwards.
	function addOverlayLayers(picker) {
		if (!picker.map.getSource("geofence-accuracy")) {
			// Added before the area itself so the buffer ring renders underneath it,
			// showing only where it pokes out beyond the actual area.
			picker.map.addSource("geofence-accuracy", {
				type: "geojson",
				data: { type: "Feature", geometry: { type: "Polygon", coordinates: [[]] } },
			});
			picker.map.addLayer({
				id: "geofence-accuracy-line", type: "line", source: "geofence-accuracy",
				paint: { "line-color": "#f59e0b", "line-width": 1.5, "line-dasharray": [1, 2], "line-opacity": 0.8 },
			});
		}

		if (!picker.map.getSource("geofence-area")) {
			picker.map.addSource("geofence-area", {
				type: "geojson",
				data: { type: "Feature", geometry: { type: "Polygon", coordinates: [[]] } },
			});
			picker.map.addLayer({
				id: "geofence-fill", type: "fill", source: "geofence-area",
				paint: { "fill-color": "#22c55e", "fill-opacity": 0.18 },
			});
			picker.map.addLayer({
				id: "geofence-line", type: "line", source: "geofence-area",
				paint: { "line-color": "#22c55e", "line-width": 2, "line-dasharray": [2, 1] },
			});
		}
	}

	// A style swap (satellite <-> streets) tears down every runtime layer,
	// Draw's own included, since setStyle replaces the whole style rather than
	// patching it. Removing and re-adding the control forces it to rebuild its
	// layers against the freshly loaded style; the feature data survives in
	// Draw's own store regardless, but is re-applied explicitly rather than
	// trusted to.
	function reattachDraw(picker) {
		if (!picker.draw) return;
		const existing = picker.draw.getAll();
		picker.map.removeControl(picker.draw);
		picker.map.addControl(picker.draw);
		if (existing.features.length) picker.draw.set(existing);
	}

	function onMapReady(picker) {
		addOverlayLayers(picker);

		if (typeof MapboxDraw !== "undefined") {
			picker.draw = new MapboxDraw({
				displayControlsDefault: false,
				controls: {},
				defaultMode: "simple_select",
				styles: drawStyles(),
			});
			picker.map.addControl(picker.draw);
			["draw.create", "draw.update"].forEach(function (name) {
				picker.map.on(name, function () { onShapeDrawn(picker); });
			});
			picker.map.on("draw.delete", function () {
				picker.ring = null;
				warn(picker, "");
				render(picker, true);
			});
			if (picker.ring) {
				picker.draw.add({ type: "Feature", properties: {},
					geometry: { type: "Polygon", coordinates: [picker.ring] } });
			}
		}

		// Fires on every later setStyle too, not just this first load, since
		// that is how Mapbox reports a finished style swap.
		picker.map.on("style.load", function () {
			addOverlayLayers(picker);
			reattachDraw(picker);
			render(picker, false);
		});

		// Admins plotting against a photograph need imagery, not just the vector
		// style, so the same satellite/streets switcher used elsewhere is wired
		// up here too.
		if (typeof MapboxStyleSwitcher !== "undefined") {
			MapboxStyleSwitcher.extend(picker.map);
		}

		const bounds = startingView(picker).bounds;
		if (bounds) picker.map.fitBounds(bounds, { padding: 40, duration: 0 });

		wire(picker);
		render(picker, false);
	}

	function onShapeDrawn(picker) {
		const features = picker.draw.getAll().features;
		const latest = features[features.length - 1];
		if (!latest || latest.geometry.type !== "Polygon") return;

		// One area per block, so an earlier shape is replaced rather than added to.
		features.slice(0, -1).forEach(function (f) { picker.draw.delete(f.id); });

		const ring = latest.geometry.coordinates[0];
		const invalid = selfIntersects(ring);
		picker.draw.setFeatureProperty(latest.id, "invalid", invalid);

		if (invalid) {
			// The shape stays on the map, coloured red by drawStyles, so the
			// crossing can be dragged straight; only the saved area is withheld
			// until it is.
			warn(picker, "That shape crosses itself, so there is no inside to test. Drag a corner to fix it.");
			picker.ring = null;
			render(picker, true);
			return;
		}

		warn(picker, "");
		picker.ring = ring;
		render(picker, true);
	}

	function wire(picker) {
		picker.root.querySelectorAll("[data-geofence-tool]").forEach(function (button) {
			button.addEventListener("click", function (event) {
				event.preventDefault();
				setTool(picker, button.dataset.geofenceTool);
			});
		});

		const slider = picker.root.querySelector("[data-geofence-radius]");
		if (slider) {
			slider.value = picker.radius;
			slider.addEventListener("input", function () {
				picker.radius = parseFloat(slider.value);
				render(picker, true);
			});
		}

		const clear = picker.root.querySelector("[data-geofence-clear]");
		if (clear) {
			clear.addEventListener("click", function (event) {
				event.preventDefault();
				if (picker.draw) picker.draw.deleteAll();
				picker.ring = null;
				warn(picker, "");
				render(picker, true);
			});
		}

		if (picker.accuracyInput) {
			picker.accuracyInput.addEventListener("input", function () { render(picker, false); });
		}

		// The pin sits at the centre of the map, which is easier to place one-handed
		// than a tap target and keeps the marker in view while panning.
		picker.map.on("move", function () {
			if (picker.tool === "pin") render(picker, true);
		});
	}

	function setTool(picker, tool) {
		picker.tool = tool;
		picker.root.querySelectorAll("[data-geofence-tool]").forEach(function (button) {
			button.classList.toggle("btn-active", button.dataset.geofenceTool === tool);
		});
		picker.root.querySelectorAll("[data-geofence-pin-only]").forEach(function (node) {
			node.classList.toggle("hidden", tool !== "pin");
		});
		picker.root.querySelectorAll("[data-geofence-shape-only]").forEach(function (node) {
			node.classList.toggle("hidden", tool !== "shape");
		});

		if (tool === "pin") {
			if (picker.draw) picker.draw.deleteAll();
			picker.ring = null;
		} else if (picker.draw) {
			// A shape stuck in the invalid (red) state still counts as drawn, even
			// though picker.ring is null for it, so its presence on the map decides
			// the mode rather than picker.ring alone.
			const hasShape = picker.draw.getAll().features.length > 0;
			picker.draw.changeMode(hasShape ? "simple_select" : "draw_polygon");
		}
		warn(picker, "");
		render(picker, true);
	}

	function warn(picker, message) {
		const el = picker.root.querySelector("[data-geofence-warning]");
		if (!el) return;
		el.textContent = message || "";
		el.classList.toggle("hidden", !message);
	}

	function render(picker, save) {
		const source = picker.map.getSource("geofence-area");
		const readout = picker.root.querySelector("[data-geofence-readout]");

		let geometry = null;
		let ring = [];

		if (picker.tool === "pin") {
			const centre = picker.map.getCenter();
			ring = circleRing(centre, picker.radius);
			geometry = {
				type: "circle",
				center: [round(centre.lng), round(centre.lat)],
				radius: picker.radius,
			};
			if (readout) {
				readout.textContent = round(centre.lat) + ", " + round(centre.lng) +
					" · " + picker.radius + " m across the radius";
			}
		} else if (picker.ring && picker.ring.length >= 4) {
			ring = picker.ring;
			geometry = {
				type: "Polygon",
				coordinates: [picker.ring.map(function (p) { return [round(p[0]), round(p[1])]; })],
			};
			if (readout) {
				readout.textContent = (picker.ring.length - 1) + " corners · " +
					humanArea(ringAreaMetres(picker.ring));
			}
		} else if (readout) {
			readout.textContent = "Draw an area on the map";
		}

		// The drawn shape is already on screen via the draw control; painting it
		// again through this layer would double the fill.
		if (source) {
			source.setData({
				type: "Feature",
				geometry: { type: "Polygon", coordinates: [picker.tool === "pin" ? ring : []] },
			});
		}

		const marker = picker.root.querySelector("[data-geofence-crosshair]");
		if (marker) marker.classList.toggle("hidden", picker.tool !== "pin");

		const accuracySource = picker.map.getSource("geofence-accuracy");
		if (accuracySource) {
			const accuracy = accuracyValue(picker);
			let bufferRing = null;
			if (picker.tool === "pin") {
				bufferRing = circleBufferRing(picker.map.getCenter(), picker.radius, accuracy);
			} else if (picker.ring && picker.ring.length >= 4) {
				bufferRing = offsetRingMetres(picker.ring, accuracy);
			}
			accuracySource.setData({
				type: "Feature",
				geometry: { type: "Polygon", coordinates: bufferRing ? [bufferRing] : [] },
			});
		}

		if (!save) return;

		const next = geometry ? JSON.stringify(geometry) : "";
		if (next === picker.input.value) return;
		picker.input.value = next;
		if (next) picker.form.dispatchEvent(new Event("change", { bubbles: true }));
	}

	function round(value) {
		return Math.round(value * 1e6) / 1e6;
	}

	function scan() {
		document.querySelectorAll(".geofence-map-uninit").forEach(init);
	}

	document.addEventListener("DOMContentLoaded", scan);
	document.body && document.body.addEventListener("htmx:afterSettle", scan);
	scan();
})();
