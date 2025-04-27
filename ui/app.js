// Base URL for API - Change this to the deployed URL when needed
const BASE_URL = "http://localhost:8080";

// DOM Elements
const navLinks = document.querySelectorAll('.nav-link');
const sections = document.querySelectorAll('.section-content');
const notification = document.getElementById('notification');
const notificationMessage = document.getElementById('notification-message');
const closeNotification = document.getElementById('close-notification');

// Navigation functionality
navLinks.forEach(link => {
    link.addEventListener('click', function(e) {
        e.preventDefault();
        
        // Remove active class from all links and sections
        navLinks.forEach(link => link.classList.remove('active'));
        sections.forEach(section => section.classList.remove('active'));
        
        // Add active class to clicked link
        this.classList.add('active');
        
        // Get the section ID from data attribute and show it
        const sectionId = this.getAttribute('data-section');
        document.getElementById(sectionId).classList.add('active');
    });
});

// Notification functionality
function showNotification(message, type = '') {
    notificationMessage.textContent = message;
    notification.className = 'notification';
    if (type) {
        notification.classList.add(type);
    }
    notification.classList.remove('hidden');
    
    // Auto hide after 5 seconds
    setTimeout(() => {
        hideNotification();
    }, 5000);
}

function hideNotification() {
    notification.classList.add('hidden');
}

closeNotification.addEventListener('click', hideNotification);

// Helper function to pretty print JSON
function formatJSON(json) {
    try {
        if (typeof json === 'string') {
            json = JSON.parse(json);
        }
        return JSON.stringify(json, null, 2);
    } catch (e) {
        console.error('Error formatting JSON:', e);
        return json;
    }
}

// Helper function to make API requests
async function apiRequest(url, method = 'GET', data = null) {
    console.log(`Making ${method} request to: ${url}`);
    
    const options = {
        method,
        headers: {
            'Content-Type': 'application/json'
        }
    };
    
    if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH')) {
        options.body = JSON.stringify(data);
        console.log(`Request body: ${options.body}`);
    }
    
    try {
        console.log(`Sending request with options:`, options);
        const response = await fetch(url, options);
        console.log(`Response status: ${response.status}`);
        
        // First get response as text for debugging
        const responseText = await response.text();
        console.log(`Response text: ${responseText}`);
        
        // Try to parse as JSON
        let responseData;
        try {
            responseData = JSON.parse(responseText);
        } catch (e) {
            console.warn(`Response is not valid JSON: ${responseText}`);
            if (response.ok) {
                // If the response is OK but not JSON, return the text
                return responseText;
            } else {
                // Otherwise, create a simple object with the text
                responseData = { message: responseText || 'Unknown error' };
            }
        }
        
        if (!response.ok) {
            const errorMessage = responseData.message || `HTTP error: ${response.status}`;
            console.error(`Request failed: ${errorMessage}`);
            throw new Error(errorMessage);
        }
        
        return responseData;
    } catch (error) {
        console.error('API Request Error:', error);
        showNotification(error.message || 'An error occurred', 'error');
        throw error;
    }
}

// Copy to clipboard function
function copyToClipboard(text) {
    navigator.clipboard.writeText(text)
        .then(() => {
            showNotification('Copied to clipboard!', 'success');
        })
        .catch(err => {
            console.error('Could not copy text: ', err);
            showNotification('Failed to copy to clipboard', 'error');
        });
}

// Subscriptions functionality
const refreshSubscriptionsBtn = document.getElementById('refresh-subscriptions');
const subscriptionsList = document.getElementById('subscriptions-list');

async function fetchSubscriptions() {
    try {
        subscriptionsList.innerHTML = '<div class="loader"></div>';
        console.log('Fetching subscriptions...');
        
        // Make the request directly to have more control over error handling
        const response = await fetch(`${BASE_URL}/api/subscriptions`);
        console.log(`Fetch subscriptions response status: ${response.status}`);
        
        if (!response.ok) {
            throw new Error(`Failed to fetch subscriptions: HTTP ${response.status}`);
        }
        
        const responseText = await response.text();
        console.log(`Subscriptions response text:`, responseText);
        
        let subscriptions;
        try {
            subscriptions = JSON.parse(responseText);
            console.log('Parsed subscriptions:', subscriptions);
        } catch (e) {
            console.error('Error parsing subscriptions JSON:', e);
            throw new Error('Invalid response format from server');
        }
        
        displaySubscriptions(subscriptions);
    } catch (error) {
        console.error('Error fetching subscriptions:', error);
        subscriptionsList.innerHTML = `<p>Failed to load subscriptions: ${error.message}</p>`;
        showNotification(`Failed to load subscriptions: ${error.message}`, 'error');
    }
}

function displaySubscriptions(subscriptions) {
    if (!subscriptions || subscriptions.length === 0) {
        subscriptionsList.innerHTML = '<p>No subscriptions found. Create one to get started.</p>';
        return;
    }
    
    subscriptionsList.innerHTML = '';
    subscriptions.forEach(subscription => {
        const subscriptionItem = document.createElement('div');
        subscriptionItem.className = 'subscription-item';
        
        const header = document.createElement('div');
        header.className = 'subscription-header';
        
        const title = document.createElement('div');
        title.className = 'subscription-title';
        title.innerHTML = `
            ${subscription.name || 'Unnamed Subscription'} 
            <span class="subscription-id" title="Click to copy" id="id-${subscription.id}">${subscription.id}</span>
            <button class="copy-id-btn" data-id="${subscription.id}" title="Copy ID">
                <i class="fas fa-copy"></i>
            </button>
        `;
        
        const actions = document.createElement('div');
        actions.className = 'subscription-actions';
        
        const toggleDetails = document.createElement('button');
        toggleDetails.className = 'subscription-details-toggle';
        toggleDetails.innerHTML = '<i class="fas fa-chevron-down"></i> Details';
        toggleDetails.addEventListener('click', function() {
            const details = subscriptionItem.querySelector('.subscription-details');
            details.classList.toggle('visible');
            this.querySelector('i').classList.toggle('fa-chevron-down');
            this.querySelector('i').classList.toggle('fa-chevron-up');
        });
        
        const editBtn = document.createElement('button');
        editBtn.className = 'btn btn-sm';
        editBtn.innerHTML = '<i class="fas fa-edit"></i> Edit';
        editBtn.addEventListener('click', () => editSubscription(subscription));
        
        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn btn-danger btn-sm';
        deleteBtn.innerHTML = '<i class="fas fa-trash"></i>';
        deleteBtn.addEventListener('click', () => deleteSubscription(subscription.id));
        
        actions.appendChild(toggleDetails);
        actions.appendChild(editBtn);
        actions.appendChild(deleteBtn);
        
        header.appendChild(title);
        header.appendChild(actions);
        
        const details = document.createElement('div');
        details.className = 'subscription-details';
        details.innerHTML = `
            <div class="json-display">
${formatJSON(subscription)}
            </div>
            <div class="subscription-buttons">
                <button class="btn btn-primary trigger-webhook-btn" data-id="${subscription.id}">
                    <i class="fas fa-paper-plane"></i> Trigger Webhook
                </button>
                <button class="btn view-tasks-btn" data-id="${subscription.id}">
                    <i class="fas fa-tasks"></i> View Tasks
                </button>
            </div>
        `;
        
        subscriptionItem.appendChild(header);
        subscriptionItem.appendChild(details);
        
        subscriptionsList.appendChild(subscriptionItem);
    });
    
    // Add event listeners to buttons in the subscription items
    document.querySelectorAll('.copy-id-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const id = this.getAttribute('data-id');
            copyToClipboard(id);
        });
    });
    
    document.querySelectorAll('.subscription-id').forEach(span => {
        span.addEventListener('click', function() {
            copyToClipboard(this.textContent);
        });
    });
    
    document.querySelectorAll('.trigger-webhook-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const subscriptionId = this.getAttribute('data-id');
            document.getElementById('trigger-subscription-id').value = subscriptionId;
            
            // Switch to analytics tab
            navLinks.forEach(link => link.classList.remove('active'));
            sections.forEach(section => section.classList.remove('active'));
            document.querySelector('[data-section="analytics"]').classList.add('active');
            document.getElementById('analytics').classList.add('active');
        });
    });
    
    document.querySelectorAll('.view-tasks-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const subscriptionId = this.getAttribute('data-id');
            document.getElementById('subscription-id-input').value = subscriptionId;
            
            // Switch to tasks tab
            navLinks.forEach(link => link.classList.remove('active'));
            sections.forEach(section => section.classList.remove('active'));
            document.querySelector('[data-section="tasks"]').classList.add('active');
            document.getElementById('tasks').classList.add('active');
            
            // Trigger tasks fetch
            document.getElementById('get-subscription-tasks').click();
        });
    });
}

function editSubscription(subscription) {
    // Fill the update form with subscription data
    document.getElementById('update-subscription-id').value = subscription.id;
    document.getElementById('update-sub-name').value = subscription.name || '';
    document.getElementById('update-target-url').value = subscription.target_url || '';
    
    let payloadText = '{}';
    try {
        payloadText = JSON.stringify(subscription.payload, null, 2);
    } catch (e) {
        console.error('Error formatting payload JSON:', e);
    }
    document.getElementById('update-payload').value = payloadText;
    
    // Show the update section
    sections.forEach(section => section.classList.remove('active'));
    document.getElementById('update-subscription').classList.add('active');
    
    // Update nav active state
    navLinks.forEach(link => link.classList.remove('active'));
}

async function deleteSubscription(id) {
    if (confirm('Are you sure you want to delete this subscription? This action cannot be undone.')) {
        try {
            // Show a loading notification
            showNotification('Deleting subscription...', 'loading');
            
            console.log(`Attempting to delete subscription with ID: ${id}`);
            
            // Make the delete request manually to handle non-JSON responses
            const response = await fetch(`${BASE_URL}/api/subscriptions/${id}`, {
                method: 'DELETE',
                headers: {
                    'Content-Type': 'application/json'
                }
            });
            
            console.log(`Delete response status: ${response.status}`);
            
            // Check if the request was successful (status 2xx)
            if (response.ok) {
                // Try to get response as text first
                const responseText = await response.text();
                console.log(`Delete response text: ${responseText}`);
                
                showNotification('Subscription deleted successfully', 'success');
                fetchSubscriptions();
            } else {
                // Try to get more information about the error
                let errorDetails = '';
                try {
                    const errorText = await response.text();
                    console.log(`Error response text: ${errorText}`);
                    
                    try {
                        // Try to parse as JSON
                        const jsonData = JSON.parse(errorText);
                        errorDetails = jsonData.message || `Status: ${response.status}`;
                    } catch (e) {
                        // Not JSON, use the raw text
                        errorDetails = errorText || `Status: ${response.status}`;
                    }
                } catch (e) {
                    errorDetails = `HTTP Status: ${response.status}`;
                }
                
                showNotification(`Failed to delete subscription: ${errorDetails}`, 'error');
                console.error('Delete error:', errorDetails);
            }
        } catch (error) {
            console.error('Delete request failed:', error);
            showNotification(`Network error deleting subscription: ${error.message}`, 'error');
        }
    }
}

refreshSubscriptionsBtn.addEventListener('click', fetchSubscriptions);

// Create Subscription functionality
const subscriptionForm = document.getElementById('subscription-form');

subscriptionForm.addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const name = document.getElementById('sub-name').value;
    const targetUrl = document.getElementById('target-url').value;
    const payloadText = document.getElementById('payload').value;
    
    let payload;
    try {
        payload = JSON.parse(payloadText);
    } catch (error) {
        showNotification('Invalid JSON in payload field', 'error');
        return;
    }
    
    const subscriptionData = {
        name,
        target_url: targetUrl,
        payload
    };
    
    try {
        const response = await apiRequest(`${BASE_URL}/api/subscriptions`, 'POST', subscriptionData);
        showNotification('Subscription created successfully', 'success');
        
        // Clear form
        subscriptionForm.reset();
        
        // Switch to subscriptions tab and refresh list
        navLinks.forEach(link => link.classList.remove('active'));
        sections.forEach(section => section.classList.remove('active'));
        document.querySelector('[data-section="subscriptions"]').classList.add('active');
        document.getElementById('subscriptions').classList.add('active');
        fetchSubscriptions();
    } catch (error) {
        showNotification('Failed to create subscription', 'error');
    }
});

// Update Subscription functionality
const updateSubscriptionForm = document.getElementById('update-subscription-form');
const cancelUpdateBtn = document.getElementById('cancel-update');

updateSubscriptionForm.addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const id = document.getElementById('update-subscription-id').value;
    const name = document.getElementById('update-sub-name').value;
    const targetUrl = document.getElementById('update-target-url').value;
    const payloadText = document.getElementById('update-payload').value;
    
    let payload;
    try {
        payload = JSON.parse(payloadText);
    } catch (error) {
        showNotification('Invalid JSON in payload field', 'error');
        return;
    }
    
    const subscriptionData = {
        name,
        target_url: targetUrl,
        payload
    };
    
    try {
        // Updated to use PATCH instead of PUT
        const response = await apiRequest(`${BASE_URL}/api/subscriptions/${id}`, 'PATCH', subscriptionData);
        showNotification('Subscription updated successfully', 'success');
        
        // Switch to subscriptions tab and refresh list
        navLinks.forEach(link => link.classList.remove('active'));
        sections.forEach(section => section.classList.remove('active'));
        document.querySelector('[data-section="subscriptions"]').classList.add('active');
        document.getElementById('subscriptions').classList.add('active');
        fetchSubscriptions();
    } catch (error) {
        showNotification('Failed to update subscription', 'error');
    }
});

cancelUpdateBtn.addEventListener('click', function() {
    // Switch back to subscriptions tab
    navLinks.forEach(link => link.classList.remove('active'));
    sections.forEach(section => section.classList.remove('active'));
    document.querySelector('[data-section="subscriptions"]').classList.add('active');
    document.getElementById('subscriptions').classList.add('active');
});

// Subscription Tasks functionality
const getSubscriptionTasksBtn = document.getElementById('get-subscription-tasks');
const subscriptionIdInput = document.getElementById('subscription-id-input');
const subscriptionTasks = document.getElementById('subscription-tasks');
const subscriptionTasksList = document.getElementById('subscription-tasks-list');

getSubscriptionTasksBtn.addEventListener('click', async function() {
    const subscriptionId = subscriptionIdInput.value.trim();
    if (!subscriptionId) {
        showNotification('Please enter a subscription ID', 'warning');
        return;
    }
    
    try {
        const tasks = await apiRequest(`${BASE_URL}/api/subscriptions/${subscriptionId}/tasks`);
        subscriptionTasksList.textContent = formatJSON(tasks);
        subscriptionTasks.classList.remove('hidden');
    } catch (error) {
        subscriptionTasks.classList.add('hidden');
    }
});

// Trigger Webhook functionality
const triggerWebhookBtn = document.getElementById('trigger-webhook');
const triggerSubscriptionId = document.getElementById('trigger-subscription-id');
const customPayload = document.getElementById('custom-payload');
const triggerResult = document.getElementById('trigger-result');
const triggerResultDetails = document.getElementById('trigger-result-details');

triggerWebhookBtn.addEventListener('click', async function() {
    const subscriptionId = triggerSubscriptionId.value.trim();
    if (!subscriptionId) {
        showNotification('Please enter a subscription ID', 'warning');
        return;
    }
    
    let payload = {};
    const payloadText = customPayload.value.trim();
    if (payloadText) {
        try {
            payload = JSON.parse(payloadText);
        } catch (error) {
            showNotification('Invalid JSON in payload field', 'error');
            return;
        }
    }
    
    try {
        const result = await apiRequest(`${BASE_URL}/api/ingest/${subscriptionId}`, 'POST', payload);
        triggerResultDetails.textContent = formatJSON(result);
        triggerResult.classList.remove('hidden');
        showNotification('Webhook triggered successfully', 'success');
    } catch (error) {
        triggerResult.classList.add('hidden');
    }
});

// Analytics functionality
const getRecentAttemptsBtn = document.getElementById('get-recent-attempts');
const analyticsSubscriptionId = document.getElementById('analytics-subscription-id');
const attemptsLimit = document.getElementById('attempts-limit');
const recentAttempts = document.getElementById('recent-attempts');
const recentAttemptsList = document.getElementById('recent-attempts-list');

getRecentAttemptsBtn.addEventListener('click', async function() {
    const subscriptionId = analyticsSubscriptionId.value.trim();
    if (!subscriptionId) {
        showNotification('Please enter a subscription ID', 'warning');
        return;
    }
    
    const limit = attemptsLimit.value;
    
    try {
        const attempts = await apiRequest(`${BASE_URL}/api/analytics/subscriptions/${subscriptionId}/recent-attempts?limit=${limit}`);
        recentAttemptsList.textContent = formatJSON(attempts);
        recentAttempts.classList.remove('hidden');
    } catch (error) {
        recentAttempts.classList.add('hidden');
    }
});

// Initialize app
document.addEventListener('DOMContentLoaded', function() {
    // Load subscriptions on initial page load
    fetchSubscriptions();
}); 