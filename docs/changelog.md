---
title: "Changelog"
sidebar: true
order: 100
tag: updated
---

# Changelog

## 6.12.0 (2025-02-01)

### Added

- [Task Block](/docs/user/blocks/task): A new block for a scavenger hunt style task that players can mark as complete.
## 6.11.0 (2025-01-28)

### Added

- New location section in the activity view for tracking locations in real-time. It answers questions like:
  - "Which teams are currently playing?"
  - "What's the average time for each location?"
  - "Which locations are the most popular right now?"
- Average duration for each location displayed in the facilitator dashboard.

### Fixed

- Fixed an issue with average durations for each location not calculating correctly.
  - A migration has been added to recalculate existing data.
- Resetting a team now redirects to the team overview page.

## 6.10.3 (2025-01-27)

### Changed

- Check-ins now appear in reverse chronological order on the team activity page.

## 6.10.2 (2025-01-27)

### Fixed

- Minor style fix on homepage
- Map rendering issue on activity page

## 6.10.1 (2025-01-26)

### Changed

- The map no longer display in Activity view if there are no mapped locations.

### Fixed

- Message to players when "Location List" navigation mode is selected.
- Removed delay in team name block.
- Increased spacing between checklist items for better readability.
- Redirect players to complete page after finishing the game.
- Corrected "check-out instead" messages.
- Added the "Show total visiting teams" setting back into the experience settings.
- The point total in the team overview now honours the Enable Points setting.

## 6.10.0 (2025-01-26)

### Added

- CLI account recovery tool for admin.

## 6.9.2 (2025-01-25)

### Fixed

- Duplicated or launched instances no longer trigger the quickstart guide.

## 6.9.1 (2025-01-18)

### Changed

- Moved `reset` and `delete` team actions to each team's overview page for better context.

### Fixed

- The Delete Service also now removes uploaded media when deleting or resetting a team or instance.
- Added missing message to leaderboard when no teams have started yet.

## 6.9.0 (2025-12-27)

### Changed

- Moved `/finish` route to `/complete`.

### Fixed

- Check-in and check-out routes now auto complete if the player is logged in.
- Minor UI fixes.

### Removed

- Check-in preview mode removed, as I'm no longer planning to add blocks to the check-in page.

## 6.8.0 (2025-12-27)

### Added

- Location edit page now shows previews for the navigation display and check in page in addition to the content preview.

### Fixed

- Blocks of a type in another branch no longer break rendering.
- Security fix: previews are only valid for templates or the owner of an instance.

## 6.7.0 (2025-12-15)

### Added

- Uploaded images are now resized using `?size=` query parameter. Supported sizes are `small`, `medium`, `large`, and `original`. Closes [#105](https://github.com/nathanhollows/Rapua/issues/105).
  - This improves performance, especially on mobile devices.

### Fixed

- Hide marker and clues icons when `Secret` routing strategy is selected.
- More buttons nows have icons for better visibility.

## 6.6.3 (2025-12-14)

### Fixed

- Markers can now be added to existing locations without coordinates. Fixes [#67](https://github.com/nathanhollows/Rapua/issues/67)

## 6.6.2 (2025-12-13)

### Fixed

- Replaced SendGrid integration with SMTP so emails work again, and reliably. Updated `.env.template` with new SMTP fields.

## 6.6.1 (2025-12-13)

### Fixed

- Start and Finish pages now render correctly in preview mode.

## 6.6.0 (2025-12-13)

### Added

- Drag and drop content blocks in the preview pane to reorder them.
- A new [Header](/docs/user/blocks/header) block with optional icons. This block can replace the default page header.
  - All locations now use the Header block for their title.
- A new [Team Name Changer](/docs/user/blocks/team-name) block that enables teams to set or change their team name on the Start page.
- New tags in docs to show new and updates content.
- New [Scheduling Games](/docs/user/scheduling-games) documentation page.
- Hovering over blocks in preview mode now highlights the matching admin block for easier identification.

### Changed

- `/lobby` renamed to `/start` in routes, services, and documentation for clarity.

### Fixed

- Minor UI fixes.
- Invalid CSRF tokens are now gracefully refreshed.
- Game scheduler now loads and saves times correctly and is rendered consistently.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.6.0)

## 6.5.1 (2025-12-12)

### Fixed

- Templates can now be launched correctly from non-owner accounts.
- The template name field is now pre-populated when launching a template.

## 6.5.0 (2025-11-29)

### Added

- Dedicated team overview page with detailed stats, progress tracking, and uploaded media gallery closes [#104](https://github.com/nathanhollows/Rapua/issues/104)

### Fixed

- Admin alerts now use default alert styles for better visibility and consistency.
- Don't show markers with coordinates (0,0) on maps to avoid confusion.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.5.0)

## 6.4.0 (2025-11-27)

### Added

- [Quiz](/docs/user/blocks/quiz) block now supports unlocked content that displays after players answer correctly

## 6.3.1 (2025-11-25)

### Fixed

- Template modal buttons now work after creating new templates
- Template preview locations now display in game structure order
- Quickstart guide dismissal no longer redirects to /admin/
- File upload limit increased to 25MB (note: nginx `client_max_body_size` must also be configured)
- Image blocks now center properly
- Removed duplicate "Locations" header in template previews
- Activity status badge now updates automatically when starting/stopping games using HTMX out-of-band swaps
- Teams page selection counter and checkboxes now update correctly when teams are added, deleted, or reset
- Fixed duplicate team entries appearing when deleting or resetting teams

## 6.3.0 (2025-11-23)

### Added

- Added a new **Secret** routing strategy for hidden bonus locations
  - See [Game Settings](/docs/user/game-settings) for user guide or [Navigation Logic Reference](/docs/developer/navigation-logic) for technical details
  - Secret locations are accessible via QR code, link, or GPS but never shown in the UI
  - Accessible when they are siblings or uncles to the current group
  - Do not affect game progression or completion
  - Automatically disable completion settings and navigation modes in admin UI

### Changed

- Location group UI now uses data-driven architecture for easier maintenance

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.3.0)

## 6.2.0 (2025-11-23)

### Added

- [Photo Block](/docs/user/blocks/photo) to allow players to take and upload photos as part of the game.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.2.0)

## 6.1.1 (2025-11-21)

### Fixed

- Fixed an issue where new instances didn't have populated game structures.

## 6.1.0 (2025-11-19)

### Added

- Delete images on block deletion to prevent orphaned files.
- Orphaned uploads cleanup job runs daily to remove unreferenced files from `static/uploads/`.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.1.0)

## 6.0.2 (2025-11-17)

### Fixed

- Buttons now support `tel:`, `sms:`, and `mailto:` links. Fixes [#99](https://github.com/nathanhollows/Rapua/issues/99)
- Location and group drag-and-drop now works after browser back navigation. Fixes [#102](https://github.com/nathanhollows/Rapua/issues/102)
- Image block uploads now work as expected. Fixes [#100](https://github.com/nathanhollows/Rapua/issues/100)
- Image blocks accept arbitrary URL sources.
- Blocks now toggle open and closed correctly in the admin interface.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.0.2)

## 6.0.1 (2025-11-15)

### Fixed

- Minor UI updates for higher contrast and mobile readability.
- Allow sms: and tel: links in content blocks. Closes [#94](https://github.com/nathanhollows/Rapua/issues/94)
- Phone preview is now an appropriate size when no content is present. Fixes [#95](https://github.com/nathanhollows/Rapua/issues/95)
- Changing the marker type on a new location no longer clears the name field.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.0.1)

## 6.0.0 (2025-11-14)

Version 6.0.0 is a *major* release introducing the game structure system.

### Added

- [Location Groups](/docs/user/location-groups) enable multi-stage games with custom routing and navigation rules [Closes #60](https://github.com/nathanhollows/Rapua/issues/60).
  - Organise locations into groups with different completion requirements (complete all, complete minimum, etc.)
  - Configure routing strategies per group (free roam, linear, random, etc.)
  - Control navigation display modes per group (show names, map, clues, etc.)
  - Auto-advance between groups or allow manual progression.
- **Game Structure Service** manages loading, saving, and validating game structures with location relations.
- Locations now show/hide blocks based on their parent group context (i.e., clues only show if enabled for the group).

### Changed

- **Breaking change:** Removed `RouteStrategy` and `NavigationDisplayMode` fields from instance settings. These are now configured per-group in the game structure.
- Navigation logic completely refactored from `NavigationService` into new `navigation` package for easier testing.
- Simplified game settings interface.
- Updated to DaisyUI 5.5.0 for improved UI components.

### Fixed

- Location display now properly respects group-level navigation settings [Closes #96](https://github.com/nathanhollows/Rapua/issues/96)

### Removed

- Instance-level routing and navigation settings (now configured per-group in game structure) [Makes #69 redundant](https://github.com/nathanhollows/Rapua/issues/69)

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v6.0.0)

## 5.2.0 (2025-10-25)

### Added

- `DuplicationService` to handle all instance, template, and location duplication with transaction safety. Fixes [#90](https://github.com/nathanhollows/Rapua/issues/90).
  - Transaction-aware repository methods: `CreateTx` for instances, instance settings, and locations; `DuplicateBlocksByOwnerTx` for blocks.
- Games can now be renamed just like templates.
  - Added `GetByID` and `Update` methods to `InstanceService`.

### Changed

- Refactored duplication logic from `InstanceService`, `LocationService`, and `TemplateService` into centralized `DuplicationService`.

### Fixed

- Instance settings are now correctly duplicated when creating or launching templates. Fixes [#87](https://github.com/nathanhollows/Rapua/issues/87).
- Nicer stripe callback pages. Fixes [#89](https://github.com/nathanhollows/Rapua/issues/89).
- Fixed v4 to v5 import references across all files.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v5.2.0)

## 5.1.0 (2025-10-23)

Version 5.1.0 introduces a very simple pricing model for Rapua. 1 credit = 1 team start per game. That's it. No tiers. No limits. No bulk discounts. A single rate for each team. Users get additional monthly credits for free. Educators get a bit more. Easy.

### Added

- Simple credit system so Rapua can be self-sufficient. See [Pricing](/pricing) for details.
- [Privacy](/privacy) updated with minor clarifying edits and [Terms and Conditions](/terms) created.
- Config package to manage environment variables and configuration settings.
- [Job scheduler](/docs/developer/job-scheduler) to manage recurring tasks.
  - Monthly job includes topping all users up with free credits.
  - Stale credit purchase clean up job
- Indicators on the games and template page to show which game is currently selected and active.

### Changed

- Colour scheme and homepage updates! The colour scheme is consistent between light and dark modes now.
- The [Experience Settings](/admin/experience) page is now much easier to read and nicer to look at.

### Fixed

- Template share link dialog no longer has duplicate URL field.
- Experience preview now shows content blocks correctly.
- Erroneous error appearing when launching a game from a template.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v5.1.0)

## 5.0.0 (2025-10-12)

v5.0.0 is a *major* release with big architectural changes to improve security, maintainability, and extensibility.

The most exciting change is the ability to use blocks in multiple contexts, not just on location pages. For example, clues have been replaced with blocks which means all sorts of content can be used as clues, not just text.

### Added

- New [Button Block](/docs/user/blocks/button) that allows admins to add styled buttons to their game.
- New [Random Clue Block](/docs/user/blocks/random-clue) that shows different players random clues from a set.
- Block context system allows blocks to be used in multiple settings, not just location pages.
  - This sets up the foundation for editing the start and end pages for custom rules and messages.
  - Also sets the foundation for using content blocks as clues.
  - A registry means developers can now fetch all blocks registered for a certain type of use. Supports multiple contexts per block.
  - Implemented `GetBlocksForContext()` and `CanBlockBeUsedInContext()` API functions for context-aware block management
  - All new `admin/blocks` views and handlers support context-aware block management and replace the old routes.
- Documentation tests now cover mandatory YAML frontmatter for all docs. Files with no yaml or missing `title` or `order` fields will break the docs service.
- Added `MustCheckOut` field to instance settings table and added migration.
- CSRF protection using gorilla/csrf middleware with session-based tokens and environment-configurable keys.

### Changed

- **Breaking change:** Removed `CompletionMethods` in favour of a `MustCheckOut` flag. The old field was obsolete with the addition of blocks.
- **Breaking change:** Removed `GetRegisteredBlocks` from blocks package in favour of context system.
- **Breaking change:** Renamed `NavigationMethod` and `NavigationMode` to `RouteStrategy` and `NavigationDisplayMode`. The original names were so bad I don't remember which one became which.
- **Breaking change:** Renamed `LocationID` to `OwnerID` on in `models.Blocks`.
- **Breaking change:** Updated blocks repository interface to use owner-based methods instead of location-based methods. `Create` method now requires `blocks.BlockContext` parameter.
- **Breaking change:** Blocks service methods migrated from location-based to owner-based paradigm (`FindByLocationID` → `FindByOwnerID`, `NewBlock` → `NewBlockWithOwnerAndContext`).
- **Breaking change:** `db.MustOpen()` now requires a `slog.Logger` parameter for structured logging.
- Checking out now happens via a button at the bottom of the location page, instead of a separate check out qr code/link.
- Updated notification message if a player tries to check out too early.
- Cookie settings for easier scanning

### Fixed

- New blocks are always saved in order, fixes [#75](https://github.com/nathanhollows/Rapua/issues/75).
- Check in view now displays correct icons and long location names no longer look crowded.
- Check out logic for tracking players at each location no longer triggers twice.
- [Quiz block](/docs/user/blocks/quiz) correctly renders if the user revisits the location after having completed the quiz.
- Minor [Broker block](/docs/user/blocks/broker) style fixes.
- Minor [Checklist block](/docs/user/blocks/checklist) style fixes.
- Minor [Sorting block](/docs/user/blocks/sorting) style fixes.

### Removed

- **Breaking change:** `Clues` models, repository, and service, views, and handlers. Fixes [#73](https://github.com/nathanhollows/Rapua/issues/73) by default.
- **Breaking change:** `Locations.Blocks` relation. Only used in tests, so switched to service calls instead.
  - Clues are now automatically migrated to [Random Clue Blocks](/docs/user/blocks/random-clue) when the migration is run.
- **Breaking change:** Legacy blocks repository methods: `FindByLocationID`, `FindBlocksAndStatesByLocationIDAndTeamCode`, `DeleteByLocationID`.
- **Breaking change:** Legacy blocks service methods: `FindByLocationID`, `NewBlock`, `FindByLocationIDAndTeamCodeWithState`.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.4.0)

## 4.3.0 (2025-09-09)

### Added

- Leaderboard service and view for admins to track teams and their points.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.3.0)

## 4.2.0 (2025-09-05)

### Added

- Password block can now show secret content after a correct answer.
- Password block is now case-insensitive and trims whitespace, making it easier for players to answer correctly.
- Pincode block can now show secret content after a correct answer.
- Textareas and text inputs now show `Optional` when not required, to help admins understand the form better.

### Fixed

- Upgraded to templ 0.3.xxx, fixed issues with LSPs and compilation. This upgrade required a move to Go 1.23.0.
- Removed duplicate form input from image block.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.2.0)

## 4.1.1 (2025-09-04)

### Fixed

- Removed duplicate drag handle icons on location list.
- Fixed an issue where location order was not saved correctly.
- Clues now use markdown.
- Add a clue button now works after deleting all clues.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.1.1)

## 4.1.0 (2025-08-11)

### Added

- Filters on locations page to show only locations without clues or markers, and full-text search.
- CSS cache busting to ensure styles are always up-to-date.

### Changed

- Upgraded from DaisyUI 4 to 5, and from TailwindCSS 3 to 4.
- Tailwind builds are now minified.

### Fixed

- Fixed incorrect view render on player check-in.
- Added early return on player start, so first error is shown.
- Build command

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.1.0)

## 4.0.2 (2025-07-23)

### Fixed

- Makefile for `build` output now works.

## 4.0.1 (2025-07-20)

### Changed

- Updated package to `v4` and migrated all imports to the new version.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.0.1)

## 4.0.0 (2025-07-20)

This update introduces significant architectural changes to make the codebase easier to maintain, test, and extend.

### Added

- `AccessService` to manage user access control and permissions.

### Changed

- **Breaking change:** Removed locationID field from Reorder method in `BlockRepository` interface. Not needed as blocks are now always associated with a location.
- **Breaking change:** Moved docs service to internal/services for consistency with other services.
- Renamed handler subpackages to `public`, `admin`, and `player` for clarity and consistency.
- Handlers now define the interfaces they depend on, rather than relying on services to provide them. This improves testability and reduces coupling. Handlers are now plumbed directly in `main.go` and passed to the server, rather than being defined in the server package. This cuts down on plumbing.
- Refactored the `GameManagerService` into `GameScheduleService`, `QuickstartService`, `InstanceSettingsService` to improve maintainability and separation of concerns.
- Refactored the `LocationService` into `InstanceSettingsService`, `LocationStatsService`, and `MarkerService`.
- Refactored the `GameplayService` into `CheckInService` and `NaivgationService` to better separate concerns and improve readability.
- Refactored out deletion logic into a new `DeleteService` to handle deletion of any entity in the system that requires cascading deletes.
- Renamed `AuthService` to `IndentityService` to better reflect its purpose, and to avoid confusion with the new `AccessService`.
- Normalised error handling across all services to use a consistent pattern, especially with missing or invalid data.
- Moved interfaces from services into handlers.

### Removed

- `ServiceResponse` is gone! Use standard Go errors instead.
- `GameplayService` and `GameManagerService` are now split into multiple services, each with a single responsibility.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v4.0.0)

## 3.10.0 (2025-07-10)

### Added

- [Clue block](/docs/user/blocks/clue)! A new interactive content block that allows players to view clues for a price.
- [Broker block](/docs/user/blocks/broker)! A new interactive content block that allows players to blindly trade points for clues.

### Changed

- Rearranged [blocks](/docs/user/blocks/) into alphabetical order in the admin interface for easier navigation.
- Images in player content are now rounded.
- Videos in docs are now rounded.

### Fixed

- **Bonus Points System**: Fixed critical issues with bonus points calculation and recording:
  - Bonus points are now calculated correctly based on visit order (first visitor gets 2x, second gets 1.5x, third gets 1.2x points)
  - CheckIn records now store the actual points awarded to players instead of just base location points
  - Implemented hybrid points system for "check in and out" mode: bonus points awarded immediately at check-in, base points awarded at check-out completion
  - CheckIn records are now updated during checkout to show total points earned from location (bonus + base points)
  - Added comprehensive test coverage for bonus points functionality
- Removed the word "Block" from [Quiz block](/docs/user/blocks/quiz) title for consistency.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.10.0)

## 3.9.2 (2025-07-02)

### Fixed

- Points now show in player footer
- Hide currently visiting in facilitators dashboard, if not used

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.9.2)

## 3.9.1 (2025-07-01)

### Fixed

- Scheduler parses dates correctly again.
- Map markers on hover now no longer steal focus on activity page.
- Indicator to set team name is now coloured.
- Notifications appear in the lobby.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.9.1)

## 3.9.0 (2025-06-29)

### Added

- [Quiz Block](/docs/user/blocks/quiz)! A new interactive content block.
- [Template](/docs/user/templates) links can now include hashes to specific locations, allowing admins to share direct links to specific parts of a game.

### Fixed

- Roadmap no longer shows completed items.
- Fixed preview mode crash when submitting correct answers to already completed blocks.
- Preview mode now correctly loads and displays instance settings (points, etc.) instead of using empty defaults.
- Lobby and next location pages now correctly handle preview mode.
- Clues now correctly save when added.
- Sorting block now correctly saves new items when added.
- Block content points are now updated live when changed in the admin interface.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.9.0)

## 3.8.1 (2025-06-18)

### Fixed

- Fixed an issue where every clue was shown to every player [#66](https://github.com/nathanhollows/Rapua/issues/66).
- Blocks now keep their order when replicated for templates [#68](https://github.com/nathanhollows/Rapua/issues/68).
- Dependent/blocking settings were not being honoured or enforced [#70](https://github.com/nathanhollows/Rapua/issues/68).
- Minor bugs in template preview rendering

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.8.1)

## 3.8.0 (2025-06-16)

### Added

- User preferences system with customizable display name, theme selection, work type field, and email sharing controls.
- Settings page for users to manage their profile and preferences.
- Password change functionality for email-authenticated users with robust validation.
- Client-side theme management using localStorage for persistent theme preferences across sessions.

### Fixed

- An issue preventing new users from creating markerless locations has been resolved.
- Fixed theme flashing on page load by applying theme settings immediately in page header.
- Locations now save clues correctly.
- Locations now accept updates without coordinates, allowing for markerless locations.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.8.0)

---

## 3.7.0 (2025-04-29)

### Added

- New [Sorting Block](/docs/user/blocks/sorting), a quiz-type block that requires players to sort items in a specific order.
- Enhanced documentation system with missing page tracking and redirect support.
- Developer docs for [Database Schema](/docs/developer/database-schema) to provide a visual representation of the database structure.
- Developer docs for [Data Flow](/docs/developer/data-flow) to illustrate how data moves through the system.

### Changed

- Minor: Admin menu on mobile now has a more consistent look and feel with the public facing menu.
- Improved heading formatting validation in documentation pages.
- Block system respects Preview mode, enabling a completed block to be recompleted in preview mode.

### Fixed

- Documentation system now properly handles redirects to maintain links between pages.
- Tailwind now scans docs for classes to ensure all styles are applied correctly.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.7.0)

---

## 3.6.0 (2025-04-20)

### Added

- Satellite map style and switcher for easier editing and viewing, especially for location with unmapped landmarks. Closes [#39](https://github.com/nathanhollows/Rapua/issues/39).

### Changed

- Minor: Anchor links in [Docs](/docs/user/) now appear slightly different to the rest of the text for better visibility.

### Fixed

- Public auth template now requires `authed` var for correct rendering. Fixes issue with incorrect buttons on facilitator dashboard.
- Admin alerts to players are now easier to dismiss and a little harder to ignore [#62](https://github.com/nathanhollows/Rapua/issues/62).

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.6.0)

---

## 3.5.0 (2025-04-19)

### Added

- **Templates System** ([#50](https://github.com/nathanhollows/Rapua/issues/50)):
  - Introduced [Templates](/docs/user/templates) for reusing game setups. Templates can be created from existing games and launched into new instances.
  - Added support for shareable template links, including unique and "unlimited" options.
  - Admins can now manage templates (create, update, delete, rename) via a dedicated interface.
  - Share links load associated content (blocks, clues, markers), with appropriate handling for expired or missing templates.

- **Preview System Enhancements**:
  - Middleware now handles previewing of routes for players, locations, navigation, and check-ins.
  - Added context keys and logic to support admin previewing without triggering other middleware layers.

- **Authentication Awareness**:
  - Public pages now detect if an admin is logged in and adjust UI accordingly (e.g., showing admin button).

- **Miscellaneous**:
  - Added unit and integration tests for affected areas, including template and instance service, middleware, and preview systems.
  - Major updates to template pages, instance lists, modals, mobile responsiveness, and admin views.
  - Added skeleton loaders for media content to enhance perceived performance.

### Changed

- Renamed **instances** to **games** across UI and documentation for clarity.
- Improved service layering and dependency isolation in the `template`, `instance`, and `user` services.
- Refactored middleware to accept minimal interfaces, improving testability and reducing coupling.
- SQLite now uses Write-Ahead Logging (WAL) mode to avoid read/write contention.
- Lobby [middleware](/docs/developer/middleware#3-lobby-middleware) now redirects and handles teams more efficiently.

### Fixed

- Cleaned up orphaned markers when locations are deleted.
- Ensured proper validation and error reporting across services.
- Fixed highlighting in navigation menus and corrected preview context logic.
- Corrected docs for [Image Blocks](/docs/user/blocks/image) to reflect new upload capabilities.
- Corrected docs for [Installation](/docs/developer/installation#installing) instructions to include folder permissions for uploads.
- Preview system fixes [#48](https://github.com/nathanhollows/Rapua/issues/48) by enabling admins to interact with closed games.
- Fixed issues identified by `golangci-lint`, specifically using `gosec`, `gocritic`, `bodyclose`, `whitespace` (\-\-fix), `misspell`, `govet`, `unused`, and `errcheck`.

### Removed

- Cleaned up debug code, placeholders, and commented-out logic.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.5.0)

---

## 3.4.0 (2025-02-11)

### Added
- **Content Blocks:**
  - [Divider Blocks](/docs/user/blocks/divider) to separate content blocks for better readability.
  - [Alert Blocks](/docs/user/blocks/alert) for important messages. 

- **Uploads and Image Management:**
  - Admins can now upload images to [Image Blocks](/docs/user/blocks/image). Images by URL are no longer supported. [#21](https://github.com/nathanhollows/Rapua/issues/21). This lays a lot of groundwork for player image and video uploads in the future ([#49](https://github.com/nathanhollows/Rapua/issues/49))

- **Game Enhancements:**
  - Introduced support and interface updates for unmapped locations. [#45](https://github.com/nathanhollows/Rapua/issues/45)
  - Added an `IsMapped` method on markers to check if a location is mapped.
  
### Changed

- Refactored URL generation, moving it out of the game manager and into the asset generator for better separation of concerns.
- Split `game_manager_service` into `instance_service` to improve maintainability.
- Refactored dependencies in services to reduce unnecessary coupling.
- Modified the quickbar to accept only an instance instead of the whole user object.

### Fixed

- Resolved an issue where duplicating locations did not copy clues and blocks. [#52](https://github.com/nathanhollows/Rapua/issues/52)
- Fixed an issue where newly created blocks did not correctly replace the location ID.
- Ensured non-marker locations are disabled from rendering on maps. [#45](https://github.com/nathanhollows/Rapua/issues/45)
- Fixed preview refresh issues after deleting a block.
- Addressed static analysis warnings for unused variables, comments, and ignored errors.
- Various minor typo fixes and documentation updates.

### Removed

- Cleaned up dead code and removed unused functions flagged by static analysis.
- Removed redundant code in templates and players.
- Removed unnecessary dependencies in services.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.4.0)

---

## 3.3.0 (2025-02-05)

### Added
- **Team Management Enhancements:**
  - Added the ability to reset a team's progress back to nothing. [#54](https://github.com/nathanhollows/Rapua/issues/54)
  - Teams page now links to the team overview for quick access.
  - API: Implemented bulk deletion of check-ins by team codes.
  - API: Introduced bulk deletion of player states by team codes.

- **Facilitator Features:**
  - A new [facilitator dashboard](/docs/user/facilitator-dashboard) helps staff know the current progress of the game, reducing the need for practice communication. [#56](https://github.com/nathanhollows/Rapua/issues/56)

- **UI/UX Improvements:**
  - [Reset and delete actions for teams](/docs/user/players-and-teams#deleting-and-resetting-teams) now require confirmation to prevent accidental data loss.
  
### Changed
- Deleting teams now refreshes the entire list view instead of partial updates for better consistency.
- Documentation improvements, including renaming files for clarity and fixing broken links.
- Improved wording and fixed minor UI inconsistencies across the platform.

### Fixed
- Ensured `instanceID` is correctly included when saving a check-in to prevent data inconsistencies.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.3.0)

---

## 3.2.0 (2025-01-31)

### Added
- Players are now redirected to a dedicated end screen upon game completion. Included an easter egg when tapping the confetti icon. [#53](https://github.com/nathanhollows/Rapua/issues/53)

### Changed
- Restyled the lobby and team name form for better clarity and user experience.
- Improved input validation and feedback messages, especially for check-in/out forms.
- Footer now includes team name, rules link, and team code for quick reference and is shown on more pages.
- Differentiated check-in elements for logged-in vs. logged-out players for a smoother experience.
- Style updates across the platform for better consistency and readability.
- Misc code refactors on internal services and handlers for readability and consistency.

### Fixed
- Fixed an issue where commas in filenames were preventing asset downloads. [#57](https://github.com/nathanhollows/Rapua/issues/57)
- Fixed an issue where the team overview failed to render when a player had visited all locations.
- Resolved issues caused by blank sessions and edge cases that led to unexpected behaviour.
- Fixed a bug where check-in/out pages weren't rendering if the player didn't have a session.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.3.0)

---

## 3.1.0 (2025-01-17)

### Added
- Instantly fit map bounds to neighbouring markers when adding locations [(#44)](https://github.com/nathanhollows/Rapua/issues/44)

- Introduced the official project logo.
- Docs for [Getting Started with Teams](/docs/user/players-and-teams) [(#43)](https://github.com/nathanhollows/Rapua/issues/43)

### Changed
- Improved team activity overview for easier browsing and better visual clarity.
- Updated a documentation icon for consistency across the interface.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.1.0)

---

## 3.0.1 (2025-01-14)

### Fixed
- Fixed an issue where the user could not switch instances. `current_instance_id` was blocked from updating in the database.

---

## 3.0.0 (2025-01-09)

### Added
- **Contact Form:**
  - Contact form messages are now sent to the platform admin instead of being thrown into the void. [#23](https://github.com/nathanhollows/Rapua/issues/23)
- **User Management:**
  - New method for deleting users programmatically, including cascading logic to clean up dependent data.
- **Documentation Enhancements:**
  - Added roadmap/wishlist for brainstorming and tracking future features.
  - Developer-specific migration documentation, including instructions, testing steps, and explanations.
  - Included a [history](/docs/user/history) of Rapua as a reference for users and developers. [#29](https://github.com/nathanhollows/Rapua/issues/29)
  - Minor changes to in-app hints and tips for better user experience.
  - Tests to ensure documentation is up-to-date and accurate; links are now checked for validity; pages must not be empty.
- **Database Migrations:**
  - Implemented a new migration system for database changes.

### Changed
- **Breaking Changes:**
  - Renamed `cmd/game-server` to `cmd/rapua` for consistency.
  - Major refactor of services and repository plumbing for better separation of concerns, maintainability, and scalability.
  - Repositories now accept a `*bun.Tx` for bulk deletions and Services now require a `db.Transactor` for beginning transactions.
  - All tests now use migrations to ensure a clean database state and non-global database vars.
  - Teams must now have UUID-based IDs for uniformity and scalability.
  - Models no longer support `deleted_at`; hard deletes are now the standard. Soft-deleted data was never used.
- **Style Updates:**
  - Docs now look better on mobile devices.
  - Submenu titles in documentation shrunk for consistency.
  - [Content Blocks](/docs/user/blocks/) are now collapsible for better readability, and auto-collapse if there are more than 3 blocks.
  - Team CheckIns are now collapsible for better readability.

### Fixed
- The confirmation modal for deleting content blocks now triggers correctly. [#36](https://github.com/nathanhollows/Rapua/issues/36)
- Resolved issues with block rendering and automatic updates. [#34](https://github.com/nathanhollows/Rapua/issues/34)
- Map markers now show the same numbers as the location list for consistency.
- Ensured user registration failures clean up partially created user data. [#40](https://github.com/nathanhollows/Rapua/issues/40)
- Registration emails are now automatically sent. [#40](https://github.com/nathanhollows/Rapua/issues/40)

### Removed
- Unused methods and global variables related to the database have been removed for better maintainability.
- Unused methods from Blocks interface removed.
- Deprecated methods for database initialization and testing replaced with updated patterns.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v3.0.0)

---

## 2.5.0 (2024-11-28)

### Added
- **Team Features:**
  - Teams can now set their own names directly within the platform.
  - Bulk deletion of teams is now supported for easier management.
- **Interactive Map:**
  - Map markers now display popups with names for better navigation and understanding of locations. [Closes #25](https://github.com/nathanhollows/Rapua/issues/25)
- **Documentation:**
  - New public documentation system [(Closes #19)](https://github.com/nathanhollows/Rapua/issues/19), including documentation:
    - User and developer guides.
    - Quickstart guide.
    - Tutorials, such as a Student Induction Tutorial.
  - "Docs" now takes the place of "Inspo" in the main navigation. [Fixes #35](https://github.com/nathanhollows/Rapua/issues/35)
- **API Updates:**
  - Introduced new endpoints for managing teams in the Teams service, allowing programmatic creation, updates, and bulk deletion.
  - Added support for creating locations using existing map markers.

### Changed
- The team management interface has been redesigned for usability, using Hyperscript for interactivity.
- Game logic refactoring ensures more efficient handling of location relationships, including clues.

### Fixed
- Resolved an issue where marker names were not populating in the activity overview.
- Corrected a database query issue with an incorrect column name.
- Fixed an issue where game relationships weren’t fully loaded, affecting progression in specific scenarios.

[Full Changelog](https://github.com/nathanhollows/Rapua/releases/tag/v2.5.0)
