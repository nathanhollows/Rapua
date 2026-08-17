// Camera scanning for the scan block, backed by the vendored zxing-wasm reader.
// The wasm is a megabyte, so it is imported on the first tap rather than at page
// load; a player who never scans never fetches it.
(function () {
	const MODULE_URL = "/static/js/zxing/reader/index.js";
	const WASM_URL = "/static/js/zxing/reader/zxing_reader.wasm";

	// Naming the formats keeps the decoder off symbologies no quest uses.
	const FORMATS = [
		"QRCode", "EAN-13", "EAN-8", "UPC-A", "UPC-E",
		"Code128", "Code39", "Code93", "ITF", "DataMatrix",
	];

	// Live video gives us many chances at a code, so each frame is decoded the
	// cheap way rather than exhaustively.
	const READER_OPTIONS = { formats: FORMATS, tryHarder: false, maxNumberOfSymbols: 1 };
	const FRAME_INTERVAL = 180;
	const FRAME_WIDTH = 640;

	const sessions = new Map();
	let readerPromise = null;

	function loadReader() {
		if (!readerPromise) {
			readerPromise = import(MODULE_URL).then(function (mod) {
				return mod.prepareZXingModule({
					overrides: { locateFile: function () { return WASM_URL; } },
					fireImmediately: true,
				}).then(function () { return mod; });
			});
			// A failed fetch must not poison every later attempt.
			readerPromise.catch(function () { readerPromise = null; });
		}
		return readerPromise;
	}

	function parts(blockID) {
		return {
			form: document.getElementById("scan-form-" + blockID),
			open: document.querySelector('[data-scan-open="' + blockID + '"]'),
			viewport: document.getElementById("scan-viewport-" + blockID),
			video: document.getElementById("scan-video-" + blockID),
			status: document.getElementById("scan-status-" + blockID),
		};
	}

	// Hidden rather than empty, so a silent status does not leave a flex gap either side.
	function say(el, message, tone) {
		if (!el) return;
		el.textContent = message || "";
		el.className = "text-sm text-center " + (!message
			? "hidden"
			: tone === "error" ? "text-error" : "opacity-70");
	}

	async function start(blockID) {
		if (sessions.has(blockID)) return;

		const el = parts(blockID);
		if (!el.form || !el.video) return;

		if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
			say(el.status, "Scanning needs a secure (https) connection. Type the code instead.", "error");
			return;
		}

		const session = { blockID, el, stopped: false, timer: null, stream: null };
		sessions.set(blockID, session);

		el.viewport.classList.remove("hidden");
		if (el.open) el.open.classList.add("hidden");
		say(el.status, "Starting camera…");

		// The permission prompt is the slow part, so the wasm downloads alongside it.
		const wanted = { video: { facingMode: { ideal: "environment" } } };
		let stream, reader;
		try {
			const settled = await Promise.all([
				navigator.mediaDevices.getUserMedia(wanted).catch(function () {
					return navigator.mediaDevices.getUserMedia({ video: true });
				}),
				loadReader(),
			]);
			stream = settled[0];
			reader = settled[1];
		} catch (err) {
			stop(blockID);
			const denied = err && (err.name === "NotAllowedError" || err.name === "SecurityError");
			say(el.status, denied
				? "Camera permission was declined. Type the code instead."
				: "The camera could not be opened. Type the code instead.", "error");
			return;
		}

		if (session.stopped) {
			stream.getTracks().forEach(function (t) { t.stop(); });
			return;
		}

		session.stream = stream;
		session.reader = reader;
		session.canvas = document.createElement("canvas");
		session.ctx = session.canvas.getContext("2d", { willReadFrequently: true });

		el.video.srcObject = stream;
		try { await el.video.play(); } catch (err) { /* autoplay races the swap */ }

		say(el.status, "Point the camera at the code.");
		tick(session);
	}

	async function tick(session) {
		if (session.stopped) return;

		const video = session.el.video;
		if (video.readyState >= 2 && video.videoWidth) {
			const width = Math.min(FRAME_WIDTH, video.videoWidth);
			const height = Math.round(video.videoHeight * (width / video.videoWidth));
			// Assigning either dimension reallocates the backing store, so only do it
			// when the camera has actually changed shape.
			if (session.canvas.width !== width || session.canvas.height !== height) {
				session.canvas.width = width;
				session.canvas.height = height;
			}
			session.ctx.drawImage(video, 0, 0, width, height);

			try {
				const results = await session.reader.readBarcodes(
					session.ctx.getImageData(0, 0, width, height), READER_OPTIONS);
				// Stop may have landed while this frame was decoding, and the frame
				// on screen when they tapped is exactly the one that would post.
				if (session.stopped) return;
				const hit = results.find(function (r) { return r.isValid && r.text; });
				if (hit) return submit(session, hit.text);
			} catch (err) {
				// A frame that will not decode is the normal case, not a failure.
			}
		}

		session.timer = setTimeout(function () { tick(session); }, FRAME_INTERVAL);
	}

	// The server decides whether it matched, so a read is only ever reported as read.
	function submit(session, text) {
		const form = session.el.form;
		const scanned = form.querySelector('input[name="scanned"]');
		const typed = form.querySelector('input[name="code"]');

		say(session.el.status, "Checking…");
		stop(session.blockID);

		if (navigator.vibrate) navigator.vibrate(40);
		if (scanned) scanned.value = text;

		// "required" guards the typed path; it would otherwise veto a camera read,
		// which leaves the box legitimately empty. The re-render restores it.
		if (typed) typed.removeAttribute("required");
		form.requestSubmit();
	}

	function stop(blockID) {
		const session = sessions.get(blockID);
		if (!session) return;
		sessions.delete(blockID);

		session.stopped = true;
		clearTimeout(session.timer);
		if (session.stream) session.stream.getTracks().forEach(function (t) { t.stop(); });

		const el = session.el;
		if (el.video) el.video.srcObject = null;
		if (el.viewport) el.viewport.classList.add("hidden");
		if (el.open) el.open.classList.remove("hidden");
	}

	document.addEventListener("click", function (event) {
		const open = event.target.closest("[data-scan-open]");
		if (open) {
			event.preventDefault();
			start(open.dataset.scanOpen);
			return;
		}
		const close = event.target.closest("[data-scan-close]");
		if (close) {
			event.preventDefault();
			stop(close.dataset.scanClose);
			say(parts(close.dataset.scanClose).status, "");
		}
	});

	// A stale "scanned" value would win over what the player just typed.
	document.addEventListener("input", function (event) {
		const field = event.target;
		if (!field.matches || !field.matches('input[name="code"]')) return;
		const scanned = field.form && field.form.querySelector('input[name="scanned"]');
		if (scanned) scanned.value = "";
	});

	// A failed validate returns an empty body, which swaps nothing, so "Checking…"
	// would sit there forever next to a camera that has already been shut off.
	document.body.addEventListener("htmx:afterRequest", function (event) {
		const form = (event.detail && event.detail.elt) || event.target;
		if (!form || !form.id || form.id.indexOf("scan-form-") !== 0) return;

		const xhr = event.detail && event.detail.xhr;
		const swapped = xhr && xhr.status < 400 && xhr.responseText.trim() !== "";
		if (swapped) return;

		const el = parts(form.id.slice("scan-form-".length));
		say(el.status, "That didn't go through. Try again.", "error");

		// submit() lifted this for the camera; no re-render came back to restore it.
		const typed = form.querySelector('input[name="code"]');
		if (typed) typed.setAttribute("required", "");
	});

	// Release the camera before htmx removes the video element underneath it. This
	// block re-renders out of band, which fires oobBeforeSwap rather than
	// beforeSwap, so both names have to be covered.
	function releaseSwappedCameras(event) {
		const target = event.detail && event.detail.target;
		if (!target) return;
		Array.from(sessions.entries()).forEach(function (entry) {
			const session = entry[1];
			if (session.el.video && target.contains(session.el.video)) stop(entry[0]);
		});
	}
	["htmx:beforeSwap", "htmx:oobBeforeSwap"].forEach(function (name) {
		document.body.addEventListener(name, releaseSwappedCameras);
	});

	window.addEventListener("pagehide", function () {
		Array.from(sessions.keys()).forEach(stop);
	});
})();
