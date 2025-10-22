// Experience preview functionality
// Manages the live preview of the player experience based on form settings

// Sample location data for preview
const locations = [
  { name: "Eiffel Tower", clue: "Find the tallest structure in Paris." },
  { name: "Statue of Liberty", clue: "Look for the statue that welcomes visitors to New York Harbor." },
  { name: "Colosseum", clue: "Find the ancient amphitheater in Rome." },
  { name: "Great Wall of China", clue: "Search for the longest wall in the world." },
  { name: "Taj Mahal", clue: "Locate the white marble mausoleum in India." }
];

// Generate random team counts for each location
const teams = Array.from({ length: locations.length }, () => Math.floor(Math.random() * 5) + 1);

/**
 * Get the data-index attribute from the currently checked radio button
 * @param {string} name - The name attribute of the radio group
 * @returns {string|null} The data-index value or null if none checked
 */
function getCheckedData(name) {
  const checkedElement = document.querySelector(`input[name="${name}"]:checked`);
  return checkedElement ? checkedElement.getAttribute("data-index") : null;
}

/**
 * Shuffle an array in place using Fisher-Yates algorithm
 * @param {Array} array - The array to shuffle
 */
function shuffleArray(array) {
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [array[i], array[j]] = [array[j], array[i]];
  }
}

/**
 * Generate HTML for a list of locations with optional team counts
 * @param {number} limit - Maximum number of locations to show
 * @returns {string} HTML string for the location list
 */
function generateLocationList(limit) {
  let html = "";
  for (let i = 0; i < limit; i++) {
    html += `<p class="text-center"><em>${locations[i].name}</em>`;
    if (document.getElementById('showTeamCount').checked) {
      html += `<br><span class="badge badge-ghost">${teams[i]} Teams Visiting</span>`;
    }
    html += `</p>`;
  }
  return html;
}

/**
 * Generate HTML for a list of location clues
 * @param {number} limit - Maximum number of clues to show
 * @returns {string} HTML string for the clue list
 */
function generateClueList(limit) {
  let html = "";
  for (let i = 0; i < limit; i++) {
    html += `<blockquote class="text-center">${locations[i].clue}</blockquote>`;
  }
  return html;
}

/**
 * Update the preview based on current form settings
 */
function updatePreview() {
  // Check if preview elements exist (they may not be present if location count is low)
  const locationListEl = document.getElementById('locationList');
  const navigationViewEl = document.getElementById('navigationView');

  if (!locationListEl || !navigationViewEl) {
    return;
  }

  // Get current form values
  const routeStrategy = getCheckedData("routeStrategy");
  const navigationDisplayMode = getCheckedData("navigationDisplayMode");
  let maxLocations = parseInt(document.getElementById('maxLocations')?.value || "0") || 0;

  let locationListHtml = "";
  let navigationViewHtml = "";

  // Route strategy: 2=Random, 1=Free Roam, 0=Ordered
  if (routeStrategy === "0") { // Random mode
    shuffleArray(locations);
  } else if (routeStrategy === "0") { // Ordered mode
    maxLocations = 1; // Only show one location at a time
  }

  // Calculate how many locations to show
  const limit = (routeStrategy === "1")
    ? locations.length // Free roam shows all
    : (maxLocations === 0 ? locations.length : Math.min(maxLocations, locations.length));

  // Navigation display modes: 0=Map Only, 1=Map+Names, 2=Names Only, 3=Clues
  switch (navigationDisplayMode) {
    case "0": // Show Map
      navigationViewHtml = '<div class="h-64 w-full bg-neutral-content rounded-lg shadow-lg flex justify-center items-center text-neutral"><em>Map</em></div>';
      break;
    case "1": // Show Map and Names
      locationListHtml = generateLocationList(limit);
      navigationViewHtml = '<div class="h-64 w-full bg-neutral-content rounded-lg shadow-lg flex justify-center items-center text-neutral"><em>Map</em></div>';
      break;
    case "2": // Show Location Names Only
      locationListHtml = generateLocationList(limit);
      break;
    case "3": // Show Clues
      navigationViewHtml = generateClueList(limit);
      break;
  }

  // Special handling for Free Roam mode (always shows all locations)
  if (routeStrategy === "1") {
    switch (navigationDisplayMode) {
      case "0": // Show Map
        navigationViewHtml = '<div class="h-64 w-full bg-neutral-content rounded-lg shadow-lg flex justify-center items-center text-neutral"><em>Map</em></div>';
        break;
      case "1": // Show Map and Names
        locationListHtml = generateLocationList(locations.length);
        navigationViewHtml = '<div class="h-64 w-full bg-neutral-content rounded-lg shadow-lg flex justify-center items-center text-neutral"><em>Map</em></div>';
        break;
      case "2": // Show Location Names Only
        locationListHtml = generateLocationList(locations.length);
        break;
      case "3": // Show Clues
        navigationViewHtml = generateClueList(locations.length);
        break;
    }
  }

  // Update the DOM
  locationListEl.innerHTML = locationListHtml;
  navigationViewEl.innerHTML = navigationViewHtml;
}

// Initialize preview on page load
updatePreview();
