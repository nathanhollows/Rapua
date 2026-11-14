// Loads Mapbox GL on first use: it is most of a megabyte and most player pages
// have no map. Pages that carry Mapbox in their head (the editor) never load this
// file, and the blocks call straight through.
(function () {
	var BASE = "https://api.mapbox.com/mapbox-gl-js/v2.10.0/mapbox-gl";
	var pending = null;

	function addScript(src) {
		return new Promise(function (resolve, reject) {
			var el = document.createElement("script");
			el.src = src;
			el.onload = resolve;
			el.onerror = function () { reject(new Error("could not load " + src)); };
			document.head.appendChild(el);
		});
	}

	window.loadMapbox = function () {
		if (window.mapboxgl && typeof MapboxStyleSwitcher !== "undefined") return Promise.resolve();
		if (!pending) {
			if (!document.querySelector("link[data-mapbox]")) {
				var css = document.createElement("link");
				css.rel = "stylesheet";
				css.href = BASE + ".css";
				css.setAttribute("data-mapbox", "");
				document.head.appendChild(css);
			}
			// The style switcher extends mapboxgl, so it loads after it.
			pending = addScript(BASE + ".js").then(function () {
				return addScript("/static/js/mapbox-satellite-view.js");
			});
			// A failed fetch must not poison every later attempt.
			pending.catch(function () { pending = null; });
		}
		return pending;
	};
})();
