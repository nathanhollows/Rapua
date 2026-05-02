---
title: "Map"
sidebar: true
order: 11
tag: new
---

# Map Block

The Map block displays an interactive Mapbox map centred on a specific location with a marker. Players can pan and zoom to explore the area, but the map does not award points and requires no interaction to complete.

## Configuration

| Field | Required | Description |
|-------|----------|-------------|
| **Location** | Yes | Drag the map to position the marker at the desired location. Latitude and longitude are saved automatically. |
| **Zoom Level** | No | Controls how close the initial view is (1–20). Higher values zoom in further. Defaults to 14 (street level). |
| **Caption** | No | Optional text displayed below the map. |

## Notes

- The player-facing map is read-only (non-interactive panning/zooming is disabled). Use a high enough zoom level to give players meaningful context.
- The block renders nothing if no location has been set (latitude and longitude are both 0).
