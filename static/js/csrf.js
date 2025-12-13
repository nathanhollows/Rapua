// CSRF Token Refresh Handler
function initCSRFRefresh() {
	let isRefreshing = false;
	let failedQueue = [];

	function processQueue(error, token = null) {
		failedQueue.forEach(promise => {
			if (error) {
				promise.reject(error);
			} else {
				promise.resolve(token);
			}
		});
		failedQueue = [];
	}

	function refreshCSRFToken() {
		if (isRefreshing) {
			return new Promise((resolve, reject) => {
				failedQueue.push({ resolve, reject });
			});
		}

		isRefreshing = true;

		return fetch('/csrf-token', {
			method: 'GET',
			credentials: 'same-origin'
		})
		.then(response => {
			if (!response.ok) {
				throw new Error('Token refresh failed');
			}
			return response.json();
		})
		.then(data => {
			const newToken = data.token;

			// Update the hx-headers attribute on body
			const body = document.body;
			const currentHeaders = body.getAttribute('hx-headers');
			if (currentHeaders) {
				const headers = JSON.parse(currentHeaders);
				headers['X-CSRF-TOKEN'] = newToken;
				body.setAttribute('hx-headers', JSON.stringify(headers));
			}

			isRefreshing = false;
			processQueue(null, newToken);
			return newToken;
		})
		.catch(error => {
			isRefreshing = false;
			processQueue(error, null);
			throw error;
		});
	}

	// Listen for htmx responseError events
	document.body.addEventListener('htmx:responseError', function(event) {
		const xhr = event.detail.xhr;

		// Check if it's a CSRF error
		// Gorilla CSRF returns "Forbidden - CSRF token invalid"
		const isCsrfError = xhr.status === 403 &&
			xhr.responseText.includes('CSRF token invalid');

		if (isCsrfError) {
			const element = event.detail.elt;

			// Check if we've already retried this element
			if (element.hasAttribute('data-csrf-retry')) {
				// Already retried once, don't retry again
				element.removeAttribute('data-csrf-retry');
				return;
			}

			event.preventDefault(); // Prevent htmx's default error handling

			// Mark that we're retrying this element
			element.setAttribute('data-csrf-retry', 'true');

			refreshCSRFToken()
				.then(newToken => {
					// Retry the original request by re-triggering the original event
					const triggeringEvent = event.detail.requestConfig.triggeringEvent;

					// Re-trigger the original event that caused the request
					htmx.trigger(element, triggeringEvent.type);

					// Clear the retry flag after a short delay (after request completes)
					setTimeout(() => {
						element.removeAttribute('data-csrf-retry');
					}, 1000);
				})
				.catch(error => {
					// Clear retry flag on error
					element.removeAttribute('data-csrf-retry');
					// Redirect to login if token refresh fails
					window.location.href = '/login';
				});
		}
	});

	// Proactive token refresh every 25 minutes (tokens expire in 30 min)
	setInterval(() => {
		refreshCSRFToken().catch(err => console.warn('Proactive CSRF refresh failed:', err));
	}, 25 * 60 * 1000);
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', initCSRFRefresh);
} else {
	// DOM already loaded
	initCSRFRefresh();
}
