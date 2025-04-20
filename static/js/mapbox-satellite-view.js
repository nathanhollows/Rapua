/**
 * MapboxStyleSwitcher - A reusable module to add a style switcher to any Mapbox map
 * Usage: 
 *   1. Include this file in your project
 *   2. Call MapboxStyleSwitcher.extend(mapInstance) on your map instance
 *   3. The style switcher controls will be added automatically to the map
 */
const MapboxStyleSwitcher = (function() {
	// Private variables and methods
	const _defaultOptions = {
		satelliteStyle: 'mapbox://styles/mapbox/satellite-streets-v12',
		controlPosition: 'top-left', // Mapbox control position: 'top-left', 'top-right', 'bottom-left', 'bottom-right'
		controlContainerClass: 'mapboxgl-ctrl mapboxgl-ctrl-group',
		switcherClass: 'map-style-switcher'
	};

	// Create the style switcher HTML elements
	function _createSwitcherElement(options) {
		// Create container for the control
		const container = document.createElement('div');
		container.className = options.controlContainerClass;

		// Create the label element
		const label = document.createElement('label');
		label.className = options.switcherClass;

		// Create the checkbox
		const checkbox = document.createElement('input');
		checkbox.type = 'checkbox';
		checkbox.className = 'tooltip tooltip-right';
		checkbox.setAttribute('aria-label', 'Toggle Satellite View');
		checkbox.dataset.tip = 'Toggle Satellite View';

		// Create the swap elements
		const swapOn = document.createElement('div');
		swapOn.className = 'swap-on switch-satellite';

		const swapOff = document.createElement('div');
		swapOff.className = 'swap-off switch-streets';

		// Assemble the control
		label.appendChild(checkbox);
		label.appendChild(swapOn);
		label.appendChild(swapOff);
		container.appendChild(label);

		return { container, checkbox };
	}

	// Create a custom Mapbox control
	function _createMapboxControl(options) {
		const elements = _createSwitcherElement(options);
		const container = elements.container;
		const checkbox = elements.checkbox;

		// Create a Mapbox GL control
		const control = {
			onAdd: function() {
				return container;
			},
			onRemove: function() {
				container.parentNode.removeChild(container);
			},
			getCheckbox: function() {
				return checkbox;
			}
		};

		return control;
	}

	// Initialize the style switcher for a given map instance
	function _initStyleSwitcher(map, options, markers) {
		// Create and add the control to the map
		const control = _createMapboxControl(options);
		map.addControl(control, options.controlPosition);

		// Get the checkbox element
		const styleToggle = control.getCheckbox();
		if (!styleToggle) return;

		// Store the initial theme style URL that was determined during map initialization
		const initialThemeStyle = map.getStyle().sprite || map.getStyle().url || '';
		const baseStyleUrl = initialThemeStyle.split('/').slice(0, -1).join('/');

		// Set initial state - we start in the initial theme, so checkbox should be checked
		// (showing satellite as the available option)
		const isCurrentlySatellite = baseStyleUrl.includes('satellite');
		styleToggle.checked = !isCurrentlySatellite;

		// Add event listener to handle style switching
		styleToggle.addEventListener('change', function() {
			// Keep track of the current camera position
			const currentCenter = map.getCenter();
			const currentZoom = map.getZoom();
			const currentBearing = map.getBearing();
			const currentPitch = map.getPitch();

			// Get current style URL
			const currentStyle = map.getStyle().sprite || map.getStyle().url || '';
			const currentlySatellite = currentStyle.includes('satellite');

			// Determine the new style based on what we're toggling to
			let newStyle;
			if (currentlySatellite) {
				// If currently satellite, switch back to initial theme
				newStyle = baseStyleUrl;
			} else {
				// If currently in initial theme, switch to satellite
				newStyle = options.satelliteStyle;
			}

			// Update the map style
			map.setStyle(newStyle);

			// When the style is loaded, restore position and markers
			map.once('style.load', function() {
				// Restore camera position
				map.setCenter(currentCenter);
				map.setZoom(currentZoom);
				map.setBearing(currentBearing);
				map.setPitch(currentPitch);

				// Re-add all markers if available
				if (markers && Array.isArray(markers)) {
					markers.forEach(marker => {
						marker.addTo(map);
					});
				}

				// Update checkbox state to show the opposite of current view
				const nowSatellite = newStyle.includes('satellite');
				styleToggle.checked = !nowSatellite;
			});
		});
	}

	// Public API
	return {
		/**
		 * Extends a Mapbox map with a style switcher
		 * @param {Object} map - The Mapbox map instance to extend
		 * @param {Object} [options] - Configuration options
		 * @param {String} [options.satelliteStyle] - Satellite style URL
		 * @param {String} [options.controlPosition] - Position of control on map ('top-left', 'top-right', etc.)
		 * @param {String} [options.controlContainerClass] - CSS class for the control container
		 * @param {String} [options.switcherClass] - CSS class for the switcher label
		 * @param {Array} [markers] - Optional array of markers to preserve when switching styles
		 */
		extend: function(map, options = {}, markers = null) {
			// Merge default options with provided options
			const mergedOptions = {
				...JSON.parse(JSON.stringify(_defaultOptions)),
				...options
			};

			// Initialize immediately if map is already loaded
			if (map.loaded()) {
				_initStyleSwitcher(map, mergedOptions, markers);
			} else {
				// Otherwise wait for map to load
				map.on('load', function() {
					_initStyleSwitcher(map, mergedOptions, markers);
				});
			}

			// Return the map for chaining
			return map;
		}
	};
})();
