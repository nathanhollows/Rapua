// Player side of the geofence block. The browser reports a position, the server
// decides whether it is inside. Whether the area itself reaches the page is the
// author's choice (GeofenceMapMode): by default it stays off, so a quest can
// hide where it wants people to go, but a block opted into the live map may
// show it too.
(function () {
	// Mirrors defaultMaxAccuracy in blocks/geofence_block.go.
	const ACCURACY_FALLBACK = 50;

	// A cold GPS fix starts vague and tightens over seconds, so the first reading
	// is watched rather than trusted. Submitting it would fail a tight area for
	// someone standing in exactly the right spot. This only guards the very
	// first fix: once one has arrived the device clearly has a GPS, and a later
	// lull (walking under trees, say) is normal rather than something to give
	// up over.
	const GIVE_UP_MS = 45000;

	// How long to sit on a miss before quietly trying again, so a search reads
	// as "still looking" rather than a string of failed attempts.
	const RECHECK_SECONDS = 5;

	const CIRCLE_STEPS = 64;
	const EARTH_METRES = 6371000;
	const FALLBACK_VIEW = { lng: 170.50283, lat: -45.87881, zoom: 13 };

	const sessions = new Map();
	// blockID -> { map, geolocate }, for blocks whose author opted into the live
	// map (GeofenceMapMode). Kept separately from `sessions`, which exists for
	// every geofence block regardless of map mode.
	const maps = new Map();

	function parts(blockID) {
		return {
			form: document.getElementById("geofence-form-" + blockID),
			check: document.querySelector('[data-geofence-check="' + blockID + '"]'),
			cancel: document.querySelector('[data-geofence-cancel="' + blockID + '"]'),
			status: document.getElementById("geofence-status-" + blockID),
		};
	}

	function say(el, message, tone) {
		if (!el) return;
		el.textContent = message || "";
		el.className = "text-sm text-center " + (!message
			? "hidden"
			: tone === "error" ? "text-error" : "opacity-70");
	}

	function limitFor(form) {
		const limit = parseFloat(form && form.dataset.maxAccuracy);
		return limit > 0 ? limit : ACCURACY_FALLBACK;
	}

	function styleForScheme() {
		const dark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
		return dark
			? "mapbox://styles/nathanhollows/cl9w3nxff002m14sy9fco4vnr"
			: "mapbox://styles/nathanhollows/clszboe2y005i01oid8ca37jm";
	}

	// A circle is stored as a centre and a radius; this is only a rendering of it.
	function circleRing(center, radiusMetres) {
		const latRadians = center.lat * Math.PI / 180;
		const dLat = (radiusMetres / EARTH_METRES) * 180 / Math.PI;
		const dLng = dLat / Math.cos(latRadians);

		const ring = [];
		for (let i = 0; i <= CIRCLE_STEPS; i++) {
			const angle = (i / CIRCLE_STEPS) * 2 * Math.PI;
			ring.push([center.lng + dLng * Math.cos(angle), center.lat + dLat * Math.sin(angle)]);
		}
		return ring;
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

	function startingView(area) {
		if (area && area.type === "circle" && area.center) {
			return { lng: area.center[0], lat: area.center[1], zoom: 16 };
		}
		if (area && area.type === "Polygon" && area.coordinates && area.coordinates[0]) {
			const ring = area.coordinates[0];
			let west = ring[0][0], east = ring[0][0], south = ring[0][1], north = ring[0][1];
			ring.forEach(function (p) {
				west = Math.min(west, p[0]); east = Math.max(east, p[0]);
				south = Math.min(south, p[1]); north = Math.max(north, p[1]);
			});
			return { lng: (west + east) / 2, lat: (south + north) / 2, zoom: 16, bounds: [[west, south], [east, north]] };
		}
		return FALLBACK_VIEW;
	}

	function drawTarget(map, area) {
		const ring = area.type === "circle"
			? circleRing({ lng: area.center[0], lat: area.center[1] }, area.radius)
			: area.coordinates[0];
		map.addSource("geofence-target", {
			type: "geojson",
			data: { type: "Feature", geometry: { type: "Polygon", coordinates: [ring] } },
		});
		map.addLayer({
			id: "geofence-target-fill", type: "fill", source: "geofence-target",
			paint: { "fill-color": "#22c55e", "fill-opacity": 0.18 },
		});
		map.addLayer({
			id: "geofence-target-line", type: "line", source: "geofence-target",
			paint: { "line-color": "#22c55e", "line-width": 2, "line-dasharray": [2, 1] },
		});
	}

	// Built, not auto-started: GeolocateControl is wired up here but only
	// triggered from start(), on the same tap that begins the check, so no
	// permission prompt fires before the player has asked for one.
	function initMap(el) {
		el.classList.remove("geofence-player-map-uninit");

		const blockID = el.id.slice("geofence-player-map-".length);
		const previous = maps.get(blockID);
		if (previous) { previous.map.remove(); maps.delete(blockID); }

		const key = document.getElementById("mapbox_key");
		if (!key || typeof mapboxgl === "undefined") return;
		mapboxgl.accessToken = key.dataset.key;

		const area = parseArea(el.dataset.area);
		const view = startingView(area);

		const map = new mapboxgl.Map({
			container: el.id,
			style: styleForScheme(),
			center: [view.lng, view.lat],
			zoom: view.zoom,
		});
		map.addControl(new mapboxgl.NavigationControl({ showCompass: false }), "top-left");

		const geolocate = new mapboxgl.GeolocateControl({
			positionOptions: { enableHighAccuracy: true },
			trackUserLocation: true,
			showUserHeading: true,
			showAccuracyCircle: true,
		});
		map.addControl(geolocate, "top-right");

		// The dot on the map and the check-in status used to come from two
		// independent watchPosition calls, updating on their own schedules and
		// agreeing with each other only by chance. When a map exists it is now
		// the one and only position source (see start()): every reading it gets
		// both moves the dot and feeds the check-in state machine below.
		geolocate.on("geolocate", function (position) {
			const session = sessions.get(blockID);
			if (session) onReading(session, position);
		});
		geolocate.on("error", function (err) {
			const session = sessions.get(blockID);
			if (session) onError(session, err);
		});

		map.on("load", function () {
			if (area) drawTarget(map, area);
			if (view.bounds) map.fitBounds(view.bounds, { padding: 40, duration: 0 });
		});

		maps.set(blockID, { map, geolocate });
	}

	function scanMaps() {
		document.querySelectorAll(".geofence-player-map-uninit").forEach(initMap);
	}

	function start(blockID) {
		if (sessions.has(blockID)) return;

		const el = parts(blockID);
		if (!el.form) return;

		if (!navigator.geolocation) {
			say(el.status, "This browser cannot report where you are.", "error");
			return;
		}

		// state moves locating -> checking -> (cooldown -> locating) on a miss,
		// or ends at complete. The watch stays open across all of it; only a
		// miss's own cooldown gap or an explicit Stop closes it.
		const session = { blockID, el, watchID: null, giveUpTimer: null, cooldownTimer: null, state: "locating", best: null };
		sessions.set(blockID, session);

		if (el.check) el.check.classList.add("hidden");
		if (el.cancel) el.cancel.classList.remove("hidden");
		say(el.status, "Finding you…");

		armGiveUp(session);

		const mapEntry = maps.get(blockID);
		if (mapEntry) {
			// Triggering starts the control's own internal watch; its "geolocate"
			// listener (wired up in initMap) feeds readings back into this session,
			// so a second watchPosition here would only duplicate it.
			mapEntry.geolocate.trigger();
		} else {
			session.watchID = navigator.geolocation.watchPosition(
				function (position) { onReading(session, position); },
				function (err) { onError(session, err); },
				{ enableHighAccuracy: true, maximumAge: 0, timeout: GIVE_UP_MS },
			);
		}
	}

	function armGiveUp(session) {
		session.giveUpTimer = setTimeout(function () {
			const best = session.best;
			const blockID = session.blockID;
			stop(blockID);
			say(parts(blockID).status, best
				? "Your phone is only getting to about ±" + Math.round(best) +
					" m here, which is too vague to check. Try moving into the open."
				: "Your phone could not get a fix. Try moving into the open.", "error");
		}, GIVE_UP_MS);
	}

	function onReading(session, position) {
		// A reading that lands mid-cooldown, or while a previous fix is still
		// being checked, is not wasted: session.best keeps tracking it, but only
		// the locating state acts on it, so the recheck cadence stays on schedule.
		const accuracy = position.coords.accuracy;
		const limit = limitFor(session.el.form);

		if (session.best === null || accuracy < session.best) session.best = accuracy;

		if (session.state !== "locating") return;

		if (accuracy > limit) {
			say(session.el.status,
				"Accurate to about ±" + Math.round(accuracy) + " m. Holding on for a better fix…");
			return;
		}

		submit(session, position);
	}

	function onError(session, err) {
		const blockID = session.blockID;
		stop(blockID);
		const el = parts(blockID);

		if (err && err.code === err.PERMISSION_DENIED) {
			say(el.status,
				"Location permission was declined. Allow it for this site, then try again.", "error");
			return;
		}
		if (err && err.code === err.POSITION_UNAVAILABLE) {
			say(el.status, "Your position is not available right now. Try moving into the open.", "error");
			return;
		}
		say(el.status, "We could not work out where you are. Try again.", "error");
	}

	// A fix worth checking is posted straight away; the watch stays open so the
	// next one is ready as soon as the verdict (or the recheck cooldown) says
	// to look again.
	function submit(session, position) {
		const form = session.el.form;
		const coords = position.coords;

		session.state = "checking";
		clearTimeout(session.giveUpTimer);
		say(session.el.status, "Checking…");

		form.querySelector('input[name="lat"]').value = coords.latitude;
		form.querySelector('input[name="lng"]').value = coords.longitude;
		form.querySelector('input[name="accuracy"]').value = Math.round(coords.accuracy);
		form.requestSubmit();
	}

	function doneEl(blockID) {
		return document.getElementById("geofence-done-" + blockID);
	}

	// The server never says how close a miss was, only whether #geofence-done-
	// <id> came back filled in, so that is the one thing worth reading here.
	function isComplete(blockID) {
		const el = doneEl(blockID);
		return !!el && !el.classList.contains("hidden");
	}

	// #geofence-live-<id> is never re-rendered (see geofence.templ), so it has
	// to be hidden from here once the done message takes its place.
	function markComplete(blockID) {
		const live = document.getElementById("geofence-live-" + blockID);
		if (live) live.classList.add("hidden");
	}

	// A miss gets a quiet countdown rather than an immediate resubmit: posting
	// on every watchPosition update would hammer the server and read as the UI
	// nagging, where a few seconds of "still looking" reads as normal search.
	function scheduleRecheck(session) {
		session.state = "cooldown";
		let remaining = RECHECK_SECONDS;
		const render = function () {
			say(session.el.status,
				"Found you. Keep this page open while you move around — rechecking in " + remaining + "…");
		};
		render();

		session.cooldownTimer = setInterval(function () {
			remaining--;
			if (remaining <= 0) {
				clearInterval(session.cooldownTimer);
				session.cooldownTimer = null;
				session.state = "locating";
				say(session.el.status, "Keep this page open while you move around.");
				return;
			}
			render();
		}, 1000);
	}

	function stop(blockID) {
		const session = sessions.get(blockID);
		if (!session) return;
		sessions.delete(blockID);

		if (session.watchID !== null) navigator.geolocation.clearWatch(session.watchID);
		clearTimeout(session.giveUpTimer);
		clearInterval(session.cooldownTimer);

		const el = session.el;
		if (el.check) el.check.classList.remove("hidden");
		if (el.cancel) el.cancel.classList.add("hidden");
	}

	// Permission is asked for on a tap, never on load: an unexplained prompt gets
	// refused, and a refusal sticks for the whole origin, poisoning every later
	// block. Where it was already granted there is nothing to explain, so those
	// players skip the tap.
	function autoStartGranted() {
		const pending = document.querySelectorAll("[data-geofence-check]");
		if (!pending.length || !navigator.permissions || !navigator.permissions.query) return;

		navigator.permissions.query({ name: "geolocation" }).then(function (status) {
			if (status.state !== "granted") return;
			pending.forEach(function (button) { start(button.dataset.geofenceCheck); });
		}).catch(function () { /* Firefox rejects for geolocation; the tap still works. */ });
	}

	document.addEventListener("click", function (event) {
		const check = event.target.closest("[data-geofence-check]");
		if (check) {
			event.preventDefault();
			start(check.dataset.geofenceCheck);
			return;
		}
		const cancel = event.target.closest("[data-geofence-cancel]");
		if (cancel) {
			event.preventDefault();
			stop(cancel.dataset.geofenceCancel);
			say(parts(cancel.dataset.geofenceCancel).status, "");
		}
	});

	// Every validate response updates #geofence-badge-<id> and #geofence-done-
	// <id> (see geofence.templ) regardless of the verdict, so a genuine network
	// or server failure is the only case left needing a distinct message here.
	document.body.addEventListener("htmx:afterRequest", function (event) {
		const form = (event.detail && event.detail.elt) || event.target;
		if (!form || !form.id || form.id.indexOf("geofence-form-") !== 0) return;

		const blockID = form.id.slice("geofence-form-".length);
		const xhr = event.detail && event.detail.xhr;

		if (!xhr || xhr.status < 200 || xhr.status >= 400) {
			stop(blockID);
			say(parts(blockID).status, "That didn't go through. Try again.", "error");
			return;
		}

		if (isComplete(blockID)) {
			stop(blockID);
			markComplete(blockID);
			return;
		}

		const session = sessions.get(blockID);
		if (session) scheduleRecheck(session);
	});

	// Release the GPS before htmx removes the block. This block re-renders out of
	// band, which fires oobBeforeSwap rather than beforeSwap.
	function releaseSwapped(event) {
		const target = event.detail && event.detail.target;
		if (!target) return;
		Array.from(sessions.entries()).forEach(function (entry) {
			if (entry[1].el.form && target.contains(entry[1].el.form)) stop(entry[0]);
		});
		// A GeolocateControl left running on a map whose block has left the DOM
		// would keep watching position, and initMap only tears down the previous
		// instance when the same block ID is rendered again, not when it is gone
		// for good.
		Array.from(maps.entries()).forEach(function (entry) {
			const container = entry[1].map.getContainer();
			if (container && target.contains(container)) {
				entry[1].map.remove();
				maps.delete(entry[0]);
			}
		});
	}
	["htmx:beforeSwap", "htmx:oobBeforeSwap"].forEach(function (name) {
		document.body.addEventListener(name, releaseSwapped);
	});

	window.addEventListener("pagehide", function () {
		Array.from(sessions.keys()).forEach(stop);
	});

	// Blocks arrive by htmx too, so newly swapped-in ones get the same treatment.
	document.body.addEventListener("htmx:afterSettle", function () {
		autoStartGranted();
		scanMaps();
	});
	autoStartGranted();
	scanMaps();
})();
