function formatDateTimeBasedOnProximity(utcDate) {
	const now = new Date();
	const localDate = new Date(utcDate);

	const oneDayMs = 24 * 60 * 60 * 1000; // Milliseconds in one day
	const oneWeekMs = 7 * oneDayMs;

	const diff = now - localDate;

	const isToday = localDate.toDateString() === now.toDateString();
	const isThisWeek = diff < oneWeekMs && !isToday;

	let formattedDate;

	if (isToday) {
		// Show only the time if the date is today
		formattedDate = new Intl.DateTimeFormat(navigator.language, {
			hour: 'numeric',
			minute: 'numeric'
		}).format(localDate);
	} else if (isThisWeek) {
		// Show weekday name and time if the date is within the last week
		formattedDate = new Intl.DateTimeFormat(navigator.language, {
			weekday: 'long',
			hour: 'numeric',
			minute: 'numeric'
		}).format(localDate);
	} else {
		// Show full date if the date is older than a week
		formattedDate = new Intl.DateTimeFormat(navigator.language, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: 'numeric'
		}).format(localDate);
	}

	return formattedDate;
}

// Function to convert UTC to local time and update the element's content
function convertUTCtoLocal() {
	const elements = document.querySelectorAll('.convert-time');

	elements.forEach((el) => {
		const utcDateStr = el.getAttribute('data-datetime');
		const localDateFormatted = formatDateTimeBasedOnProximity(utcDateStr);
		el.textContent = localDateFormatted;
	});
}

// Run the conversion when the document loads
window.addEventListener('DOMContentLoaded', convertUTCtoLocal);
// Also trigger the conversion after any HTMX request has settled (i.e., after content is swapped)
window.addEventListener('htmx:afterSettle', convertUTCtoLocal);

// Paste to Markdown on textarea
document.addEventListener("DOMContentLoaded", function() {
	const turndownService = new TurndownService({
		headingStyle: "atx",
		bulletListMarker: "-",
		codeBlockStyle: "fenced",
		emDelimiter: "*",
		strongDelimiter: "**",
		linkStyle: "inlined",
	});

	// Function to initialize markdown conversion for a textarea
	function initializeMarkdownConversion(textarea) {
		if (textarea.dataset.processed === "true") {
			return; // Prevent multiple initializations
		}
		textarea.dataset.processed = "true";

		// Handle text input to convert to HTML (optional, you may remove this if you only want paste handling)
		// textarea.addEventListener("input", function() {
		// 	const markdownText = textarea.value;
		// 	const html = turndownService.turndown(markdownText);
		// });

		// Handle paste event
		textarea.addEventListener("paste", function(event) {
			event.preventDefault(); // Prevent default paste action

			const clipboardData = (event.clipboardData || window.clipboardData);
			const pastedData = clipboardData.getData('text/html') || clipboardData.getData('text/plain');

			// Convert the pasted HTML or plain text to Markdown
			let markdown = pastedData;

			// If HTML is pasted, convert it
			if (clipboardData.getData('text/html')) {
				markdown = turndownService.turndown(pastedData);
			}

			// Insert the Markdown at the cursor position using execCommand for undo support
			insertMarkdownAtCursor(textarea, markdown);
		});
	}

	// Function to insert text at the cursor position while maintaining undo functionality
	function insertMarkdownAtCursor(textarea, markdown) {
		textarea.focus(); // Focus the textarea
		const selection = window.getSelection();
		if (document.queryCommandSupported("insertText")) {
			// Use the execCommand for inserting text to keep it in the undo stack
			document.execCommand("insertText", false, markdown);
		} else {
			// Fallback for browsers not supporting execCommand
			const start = textarea.selectionStart;
			const end = textarea.selectionEnd;

			const before = textarea.value.substring(0, start);
			const after = textarea.value.substring(end);

			// Set the new value with the markdown inserted
			textarea.value = before + markdown + after;

			// Set the new cursor position after the inserted markdown
			const newPosition = start + markdown.length;
			textarea.setSelectionRange(newPosition, newPosition);
		}
	}

	// Select and initialize any existing textareas
	document.querySelectorAll(".markdown-textarea").forEach(initializeMarkdownConversion);

	// MutationObserver to monitor DOM changes for new textareas
	const observer = new MutationObserver((mutations) => {
		mutations.forEach((mutation) => {
			if (mutation.type === "childList" && mutation.addedNodes.length > 0) {
				mutation.addedNodes.forEach((node) => {
					if (node.nodeType === Node.ELEMENT_NODE) {
						// Check for new textareas with the markdown-textarea class
						if (node.matches(".markdown-textarea")) {
							initializeMarkdownConversion(node);
						}

						// If new children are added that may contain textareas
						node.querySelectorAll(".markdown-textarea").forEach(initializeMarkdownConversion);
					}
				});
			}
		});
	});

	// Start observing the document body for changes
	observer.observe(document.body, { childList: true, subtree: true });
});

// Textarea shortcuts
(function() {
	function enhanceMarkdownTextareas() {
		var textareas = document.querySelectorAll('textarea.markdown-textarea:not([data-md-enhanced])');

		textareas.forEach(function(textarea) {
			// Mark this textarea as enhanced
			textarea.setAttribute('data-md-enhanced', 'true');

			textarea.addEventListener('keydown', function(e) {
				if (isCtrlOrCmdPressed(e)) {
					if (e.key === 'b' || e.key === 'B') {
						e.preventDefault();
						handleBold(this);
					} else if (e.key === 'i' || e.key === 'I') {
						e.preventDefault();
						handleItalics(this);
					} else if (e.key === 'k' || e.key === 'K') {
						e.preventDefault();
						handleLink(this);
					}
				} else if (e.key === 'Enter') {
					handleListSupport(this, e);
				}
			});
		});
	}

	function isCtrlOrCmdPressed(e) {
		return e.ctrlKey || e.metaKey;
	}

	function handleBold(textarea) {
		applyFormatting(textarea, '**');
	}

	function handleItalics(textarea) {
		applyFormatting(textarea, '*');
	}

	function applyFormatting(textarea, marker) {
		var start = textarea.selectionStart;
		var end = textarea.selectionEnd;
		var text = textarea.value;

		// Expand selection to include existing markers if they are present
		var before = text.substring(0, start);
		var after = text.substring(end);

		var selection = text.substring(start, end);
		var expandedStart = start;
		var expandedEnd = end;

		// Check for existing markers before and after the selection
		if (before.endsWith(marker) && after.startsWith(marker)) {
			// Remove formatting
			expandedStart -= marker.length;
			expandedEnd += marker.length;
			textarea.setSelectionRange(expandedStart, expandedEnd);
			textarea.setRangeText(selection, expandedStart, expandedEnd, 'start');
			textarea.setSelectionRange(expandedStart, expandedStart + selection.length);
		} else {
			// Add formatting
			textarea.setRangeText(marker + selection + marker, start, end, 'end');
			textarea.setSelectionRange(start + marker.length, start + marker.length + selection.length);
		}
	}

	function handleLink(textarea) {
		var start = textarea.selectionStart;
		var end = textarea.selectionEnd;
		var text = textarea.value;
		var selectedText = text.substring(start, end);

		// Regular expression to find Markdown links
		var linkRegex = /\[([^\]]+)\]\(([^\)]+)\)/g;
		var match;
		var linkStart, linkEnd;
		var inLink = false;

		// Check if cursor is inside a link
		linkRegex.lastIndex = 0;
		while ((match = linkRegex.exec(text)) !== null) {
			var matchStart = match.index;
			var matchEnd = linkRegex.lastIndex;
			if (start >= matchStart && end <= matchEnd) {
				inLink = true;
				linkStart = matchStart;
				linkEnd = matchEnd;
				selectedText = match[1]; // Text inside the link
				break;
			}
		}

		if (inLink) {
			// Remove link formatting
			textarea.setSelectionRange(linkStart, linkEnd);
			textarea.setRangeText(selectedText, linkStart, linkEnd, 'start');
			// Place cursor after the unlinked text
			textarea.setSelectionRange(linkStart + selectedText.length, linkStart + selectedText.length);
		} else {
			if (selectedText === '') {
				// Insert placeholder link
				var placeholder = '[text](url)';
				textarea.setRangeText(placeholder, start, end, 'end');
				textarea.setSelectionRange(start + 1, start + 5); // Select 'text'
			} else {
				// Surround selected text with link syntax
				var newText = '[' + selectedText + '](url)';
				textarea.setRangeText(newText, start, end, 'end');
				// Place cursor inside 'url'
				var urlStart = start + newText.indexOf('](url)') + 2; // Position after ']('
				var urlEnd = urlStart + 3; // 'url' is 3 letters
				textarea.setSelectionRange(urlStart, urlEnd);
			}
		}
	}

	function handleListSupport(textarea, e) {
		var start = textarea.selectionStart;
		var text = textarea.value;

		var lineStart = text.lastIndexOf('\n', start - 1) + 1;
		var lineEnd = text.indexOf('\n', start);
		if (lineEnd === -1) lineEnd = text.length;

		var lineText = text.substring(lineStart, lineEnd);
		var trimmedLine = lineText.trimLeft();
		var leadingWhitespace = lineText.substring(0, lineText.length - trimmedLine.length);

		var markerMatch = trimmedLine.match(/^([-*]|\d+\.)\s+/);
		if (markerMatch) {
			var afterMarker = trimmedLine.substring(markerMatch[0].length);
			if (afterMarker.length === 0) {
				// Line contains only the list marker, remove it
				e.preventDefault();
				var beforeLine = text.substring(0, lineStart);
				var afterLine = text.substring(lineEnd);
				var newCursorPos = lineStart;
				textarea.setRangeText('', lineStart, lineEnd + 1, 'start');
				textarea.setSelectionRange(newCursorPos, newCursorPos);
			} else {
				e.preventDefault();
				var marker = markerMatch[1]; // e.g., '-', '*', '1.'
				var spaces = markerMatch[0].substring(marker.length); // spaces after marker
				var newMarker = marker;
				var olMatch = marker.match(/^(\d+)\.$/);
				if (olMatch) {
					var num = parseInt(olMatch[1], 10);
					newMarker = (num + 1) + '.';
				}
				var newLine = '\n' + leadingWhitespace + newMarker + spaces;
				var newCursorPos = start + newLine.length;
				textarea.setRangeText(newLine, start, start, 'end');
				textarea.setSelectionRange(newCursorPos, newCursorPos);
			}
		}
	}

	// Run on document ready
	document.addEventListener('DOMContentLoaded', function() {
		enhanceMarkdownTextareas();
		// Run after any htmx request
		document.body.addEventListener('htmx:afterSwap', function() {
			enhanceMarkdownTextareas();
		});
	});

})();
