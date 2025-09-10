/**
 * Modern JavaScript class with comprehensive JSDoc, ES6+ features, and decorators.
 * This module demonstrates advanced JavaScript patterns including private fields,
 * async/await, decorators, mixins, and comprehensive documentation.
 * 
 * @fileoverview Advanced user management system with modern JavaScript features
 * @version 2.1.0
 * @author Development Team
 * @since 2023-01-01
 */

import { EventEmitter } from 'events';
import { performance } from 'perf_hooks';

/**
 * @typedef {Object} UserData
 * @property {string} id - Unique identifier
 * @property {string} email - User email address
 * @property {string} firstName - User's first name
 * @property {string} lastName - User's last name
 * @property {boolean} isActive - Whether user is active
 * @property {boolean} isAdmin - Whether user has admin privileges
 * @property {Date} createdAt - User creation timestamp
 * @property {Date} updatedAt - Last update timestamp
 * @property {UserProfile} [profile] - Optional user profile data
 */

/**
 * @typedef {Object} UserProfile
 * @property {string} [bio] - User biography
 * @property {string} [avatarUrl] - Profile picture URL
 * @property {string} [website] - Personal website
 * @property {string} [location] - User location
 * @property {string} [phone] - Phone number
 * @property {Object} [preferences] - User preferences
 */

/**
 * @typedef {Object} ValidationResult
 * @property {boolean} isValid - Whether validation passed
 * @property {string[]} errors - Array of validation errors
 * @property {Object} [data] - Processed/sanitized data
 */

/**
 * @typedef {Object} QueryOptions
 * @property {number} [limit=50] - Maximum results to return
 * @property {number} [offset=0] - Number of results to skip
 * @property {string} [sortBy='createdAt'] - Field to sort by
 * @property {('asc'|'desc')} [sortOrder='desc'] - Sort direction
 * @property {Object} [filters] - Additional filters
 */

// Decorator functions for class methods

/**
 * Performance monitoring decorator
 * @param {Function} target - The target function
 * @param {string} propertyKey - Method name
 * @param {PropertyDescriptor} descriptor - Method descriptor
 * @returns {PropertyDescriptor} Modified descriptor with performance monitoring
 */
function performanceMonitor(target, propertyKey, descriptor) {
    const originalMethod = descriptor.value;
    
    descriptor.value = async function(...args) {
        const startTime = performance.now();
        const methodName = `${this.constructor.name}.${propertyKey}`;
        
        console.time(methodName);
        
        try {
            const result = await originalMethod.apply(this, args);
            const duration = performance.now() - startTime;
            
            console.timeEnd(methodName);
            console.log(`📊 ${methodName} executed in ${duration.toFixed(2)}ms`);
            
            return result;
        } catch (error) {
            const duration = performance.now() - startTime;
            console.timeEnd(methodName);
            console.error(`❌ ${methodName} failed after ${duration.toFixed(2)}ms:`, error);
            throw error;
        }
    };
    
    return descriptor;
}

/**
 * Method validation decorator
 * @param {Object} validationRules - Validation rules for parameters
 * @returns {Function} Decorator function
 */
function validate(validationRules) {
    return function(target, propertyKey, descriptor) {
        const originalMethod = descriptor.value;
        
        descriptor.value = function(...args) {
            // Perform validation based on rules
            for (const [index, rule] of Object.entries(validationRules)) {
                const arg = args[parseInt(index)];
                
                if (rule.required && (arg === undefined || arg === null)) {
                    throw new Error(`Parameter ${index} is required for ${propertyKey}`);
                }
                
                if (rule.type && arg !== undefined && typeof arg !== rule.type) {
                    throw new Error(`Parameter ${index} must be of type ${rule.type}`);
                }
                
                if (rule.validate && !rule.validate(arg)) {
                    throw new Error(`Parameter ${index} failed validation for ${propertyKey}`);
                }
            }
            
            return originalMethod.apply(this, args);
        };
        
        return descriptor;
    };
}

/**
 * Caching decorator for methods
 * @param {number} ttlMs - Cache TTL in milliseconds
 * @returns {Function} Decorator function
 */
function cache(ttlMs = 60000) {
    return function(target, propertyKey, descriptor) {
        const originalMethod = descriptor.value;
        const cacheKey = `${target.constructor.name}.${propertyKey}`;
        const cache = new Map();
        
        descriptor.value = function(...args) {
            const key = JSON.stringify(args);
            const cached = cache.get(key);
            
            if (cached && (Date.now() - cached.timestamp) < ttlMs) {
                console.log(`🎯 Cache hit for ${cacheKey}`);
                return cached.value;
            }
            
            const result = originalMethod.apply(this, args);
            cache.set(key, { value: result, timestamp: Date.now() });
            
            console.log(`💾 Cached result for ${cacheKey}`);
            return result;
        };
        
        return descriptor;
    };
}

/**
 * Mixin for adding observable functionality to classes
 * @param {Function} BaseClass - Base class to extend
 * @returns {Function} Extended class with observable functionality
 */
const ObservableMixin = (BaseClass) => class extends BaseClass {
    constructor(...args) {
        super(...args);
        this._observers = new Map();
    }
    
    /**
     * Add an observer for a specific event
     * @param {string} event - Event name
     * @param {Function} callback - Callback function
     * @returns {Function} Unsubscribe function
     */
    observe(event, callback) {
        if (!this._observers.has(event)) {
            this._observers.set(event, new Set());
        }
        
        this._observers.get(event).add(callback);
        
        // Return unsubscribe function
        return () => {
            this._observers.get(event)?.delete(callback);
        };
    }
    
    /**
     * Emit an event to all observers
     * @param {string} event - Event name
     * @param {*} data - Event data
     */
    emit(event, data) {
        const observers = this._observers.get(event);
        if (observers) {
            observers.forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    console.error(`Observer error for event ${event}:`, error);
                }
            });
        }
    }
};

/**
 * Mixin for adding caching functionality to classes
 * @param {Function} BaseClass - Base class to extend
 * @returns {Function} Extended class with caching functionality
 */
const CacheableMixin = (BaseClass) => class extends BaseClass {
    constructor(...args) {
        super(...args);
        this._cache = new Map();
        this._cacheTtl = 300000; // 5 minutes default
    }
    
    /**
     * Get cached value or compute if not cached
     * @param {string} key - Cache key
     * @param {Function} computeFn - Function to compute value if not cached
     * @param {number} [ttl] - Custom TTL for this key
     * @returns {*} Cached or computed value
     */
    getCached(key, computeFn, ttl = this._cacheTtl) {
        const cached = this._cache.get(key);
        
        if (cached && (Date.now() - cached.timestamp) < ttl) {
            return cached.value;
        }
        
        const value = computeFn();
        this._cache.set(key, { value, timestamp: Date.now() });
        
        return value;
    }
    
    /**
     * Clear cache entries (all or by pattern)
     * @param {string|RegExp} [pattern] - Pattern to match keys
     */
    clearCache(pattern) {
        if (!pattern) {
            this._cache.clear();
            return;
        }
        
        for (const key of this._cache.keys()) {
            if (pattern instanceof RegExp ? pattern.test(key) : key.includes(pattern)) {
                this._cache.delete(key);
            }
        }
    }
};

/**
 * Advanced User Management System
 * 
 * This class provides comprehensive user management functionality with:
 * - Private fields for data encapsulation
 * - Async/await for non-blocking operations
 * - Event-driven architecture
 * - Comprehensive validation
 * - Performance monitoring
 * - Caching mechanisms
 * - Modern JavaScript features
 * 
 * @class
 * @extends {ObservableMixin(CacheableMixin(EventEmitter))}
 * @example
 * ```javascript
 * const userManager = new AdvancedUserManager({
 *   maxUsers: 10000,
 *   cacheTimeout: 300000
 * });
 * 
 * await userManager.initialize();
 * 
 * const user = await userManager.createUser({
 *   email: 'john@example.com',
 *   firstName: 'John',
 *   lastName: 'Doe'
 * });
 * ```
 */
class AdvancedUserManager extends ObservableMixin(CacheableMixin(EventEmitter)) {
    // Private fields using the private field syntax
    #users = new Map();
    #userIndex = new Map();
    #validators = new Map();
    #isInitialized = false;
    #config = {
        maxUsers: 10000,
        cacheTimeout: 300000,
        enableMetrics: true,
        autoBackup: true
    };
    #metrics = {
        totalOperations: 0,
        successfulOperations: 0,
        failedOperations: 0,
        averageResponseTime: 0
    };
    #backupInterval = null;
    
    // Static properties for class-level configuration
    static DEFAULT_CONFIG = {
        maxUsers: 10000,
        cacheTimeout: 300000,
        enableMetrics: true,
        autoBackup: true
    };
    
    static VERSION = '2.1.0';
    static SUPPORTED_OPERATIONS = ['create', 'read', 'update', 'delete', 'search', 'batch'];
    
    /**
     * Create an AdvancedUserManager instance
     * @param {Object} [config={}] - Configuration options
     * @param {number} [config.maxUsers=10000] - Maximum number of users
     * @param {number} [config.cacheTimeout=300000] - Cache timeout in milliseconds
     * @param {boolean} [config.enableMetrics=true] - Enable performance metrics
     * @param {boolean} [config.autoBackup=true] - Enable automatic backup
     * @throws {Error} When invalid configuration is provided
     */
    constructor(config = {}) {
        super();
        
        // Merge configuration with defaults
        this.#config = { ...AdvancedUserManager.DEFAULT_CONFIG, ...config };
        
        // Validate configuration
        this.#validateConfig();
        
        // Set up event listeners
        this.#setupEventListeners();
        
        // Initialize validators
        this.#initializeValidators();
        
        console.log(`🚀 AdvancedUserManager v${AdvancedUserManager.VERSION} initialized`);
    }
    
    /**
     * Get the current configuration
     * @type {Object}
     * @readonly
     */
    get config() {
        return { ...this.#config };
    }
    
    /**
     * Get current metrics
     * @type {Object}
     * @readonly
     */
    get metrics() {
        return { ...this.#metrics };
    }
    
    /**
     * Get initialization status
     * @type {boolean}
     * @readonly
     */
    get isInitialized() {
        return this.#isInitialized;
    }
    
    /**
     * Get total number of users
     * @type {number}
     * @readonly
     */
    get userCount() {
        return this.#users.size;
    }
    
    /**
     * Check if the user manager has reached capacity
     * @type {boolean}
     * @readonly
     */
    get isAtCapacity() {
        return this.userCount >= this.#config.maxUsers;
    }
    
    /**
     * Initialize the user manager with async operations
     * @async
     * @returns {Promise<void>}
     * @throws {Error} When initialization fails
     * @example
     * ```javascript
     * await userManager.initialize();
     * ```
     */
    @performanceMonitor
    async initialize() {
        if (this.#isInitialized) {
            console.warn('⚠️  UserManager already initialized');
            return;
        }
        
        try {
            console.log('🔄 Initializing UserManager...');
            
            // Simulate async initialization (database connection, etc.)
            await this.#simulateAsyncOperation('initialization', 100);
            
            // Set up periodic backup if enabled
            if (this.#config.autoBackup) {
                this.#backupInterval = setInterval(() => {
                    this.#performBackup();
                }, 60000); // Backup every minute
            }
            
            this.#isInitialized = true;
            this.emit('initialized', { timestamp: new Date() });
            
            console.log('✅ UserManager initialization complete');
        } catch (error) {
            console.error('❌ UserManager initialization failed:', error);
            throw new Error(`Initialization failed: ${error.message}`);
        }
    }
    
    /**
     * Create a new user with comprehensive validation
     * @async
     * @param {Object} userData - User data object
     * @param {string} userData.email - User email (required)
     * @param {string} userData.firstName - First name (required)
     * @param {string} userData.lastName - Last name (required)
     * @param {UserProfile} [userData.profile] - Optional profile data
     * @param {Object} [options={}] - Creation options
     * @param {boolean} [options.skipValidation=false] - Skip validation
     * @param {boolean} [options.sendWelcomeEmail=true] - Send welcome email
     * @returns {Promise<UserData>} Created user object
     * @throws {Error} When validation fails or user creation fails
     * @example
     * ```javascript
     * const user = await userManager.createUser({
     *   email: 'jane@example.com',
     *   firstName: 'Jane',
     *   lastName: 'Smith',
     *   profile: {
     *     bio: 'Software developer',
     *     location: 'San Francisco'
     *   }
     * });
     * ```
     */
    @performanceMonitor
    @validate({
        0: { required: true, type: 'object' }
    })
    async createUser(userData, options = {}) {
        this.#ensureInitialized();
        
        if (this.isAtCapacity) {
            throw new Error(`Maximum user limit (${this.#config.maxUsers}) reached`);
        }
        
        const startTime = performance.now();
        
        try {
            // Validate user data
            if (!options.skipValidation) {
                const validation = await this.#validateUserData(userData);
                if (!validation.isValid) {
                    throw new Error(`Validation failed: ${validation.errors.join(', ')}`);
                }
                userData = validation.data;
            }
            
            // Check for duplicate email
            if (this.#userIndex.has(userData.email.toLowerCase())) {
                throw new Error(`User with email ${userData.email} already exists`);
            }
            
            // Generate unique ID
            const userId = this.#generateUserId();
            
            // Create user object with metadata
            const user = {
                id: userId,
                email: userData.email.toLowerCase(),
                firstName: userData.firstName,
                lastName: userData.lastName,
                isActive: true,
                isAdmin: false,
                createdAt: new Date(),
                updatedAt: new Date(),
                profile: userData.profile || {},
                ...userData
            };
            
            // Store user
            this.#users.set(userId, user);
            this.#userIndex.set(user.email, userId);
            
            // Update metrics
            this.#updateMetrics('create', true, performance.now() - startTime);
            
            // Emit events
            this.emit('userCreated', user);
            this.emit('user:create', { user, timestamp: new Date() });
            
            // Send welcome email if requested
            if (options.sendWelcomeEmail !== false) {
                await this.#sendWelcomeEmail(user);
            }
            
            console.log(`✅ Created user: ${user.email} (ID: ${userId})`);
            
            return { ...user }; // Return a copy to prevent external mutations
            
        } catch (error) {
            this.#updateMetrics('create', false, performance.now() - startTime);
            console.error('❌ User creation failed:', error);
            throw error;
        }
    }
    
    /**
     * Retrieve a user by ID or email
     * @async
     * @param {string} identifier - User ID or email address
     * @param {Object} [options={}] - Retrieval options
     * @param {boolean} [options.includeProfile=true] - Include profile data
     * @param {boolean} [options.useCache=true] - Use cached data if available
     * @returns {Promise<UserData|null>} User object or null if not found
     * @example
     * ```javascript
     * const user = await userManager.getUser('user123');
     * const userByEmail = await userManager.getUser('jane@example.com');
     * ```
     */
    @performanceMonitor
    @cache(60000) // Cache for 1 minute
    async getUser(identifier, options = {}) {
        this.#ensureInitialized();
        
        const cacheKey = `user:${identifier}`;
        
        if (options.useCache !== false) {
            const cached = this.getCached(cacheKey, () => this.#getUserInternal(identifier));
            if (cached) {
                return cached;
            }
        }
        
        const user = this.#getUserInternal(identifier);
        
        if (user && !options.includeProfile) {
            const { profile, ...userWithoutProfile } = user;
            return userWithoutProfile;
        }
        
        return user;
    }
    
    /**
     * Update user information
     * @async
     * @param {string} identifier - User ID or email
     * @param {Object} updateData - Data to update
     * @param {Object} [options={}] - Update options
     * @param {boolean} [options.skipValidation=false] - Skip validation
     * @param {boolean} [options.createSnapshot=true] - Create backup snapshot
     * @returns {Promise<UserData>} Updated user object
     * @throws {Error} When user not found or validation fails
     * @example
     * ```javascript
     * const updatedUser = await userManager.updateUser('user123', {
     *   firstName: 'John',
     *   profile: {
     *     bio: 'Updated bio'
     *   }
     * });
     * ```
     */
    @performanceMonitor
    async updateUser(identifier, updateData, options = {}) {
        this.#ensureInitialized();
        
        const user = this.#getUserInternal(identifier);
        if (!user) {
            throw new Error(`User not found: ${identifier}`);
        }
        
        const startTime = performance.now();
        
        try {
            // Create snapshot if requested
            if (options.createSnapshot !== false) {
                await this.#createUserSnapshot(user);
            }
            
            // Validate update data
            if (!options.skipValidation) {
                const validation = await this.#validateUserData(updateData, true);
                if (!validation.isValid) {
                    throw new Error(`Validation failed: ${validation.errors.join(', ')}`);
                }
                updateData = validation.data;
            }
            
            // Apply updates
            const updatedUser = {
                ...user,
                ...updateData,
                updatedAt: new Date()
            };
            
            // Handle email change
            if (updateData.email && updateData.email !== user.email) {
                this.#userIndex.delete(user.email);
                this.#userIndex.set(updateData.email.toLowerCase(), user.id);
            }
            
            // Update storage
            this.#users.set(user.id, updatedUser);
            
            // Clear relevant caches
            this.clearCache(user.id);
            this.clearCache(user.email);
            
            // Update metrics
            this.#updateMetrics('update', true, performance.now() - startTime);
            
            // Emit events
            this.emit('userUpdated', { user: updatedUser, changes: updateData });
            this.emit('user:update', { 
                user: updatedUser, 
                changes: updateData, 
                timestamp: new Date() 
            });
            
            console.log(`✅ Updated user: ${user.email}`);
            
            return { ...updatedUser };
            
        } catch (error) {
            this.#updateMetrics('update', false, performance.now() - startTime);
            console.error('❌ User update failed:', error);
            throw error;
        }
    }
    
    /**
     * Delete a user (soft delete by default)
     * @async
     * @param {string} identifier - User ID or email
     * @param {Object} [options={}] - Deletion options
     * @param {boolean} [options.hardDelete=false] - Permanently delete user
     * @param {string} [options.reason] - Reason for deletion
     * @returns {Promise<boolean>} Success status
     * @throws {Error} When user not found
     * @example
     * ```javascript
     * await userManager.deleteUser('user123', { 
     *   reason: 'Account deactivation requested' 
     * });
     * ```
     */
    @performanceMonitor
    async deleteUser(identifier, options = {}) {
        this.#ensureInitialized();
        
        const user = this.#getUserInternal(identifier);
        if (!user) {
            throw new Error(`User not found: ${identifier}`);
        }
        
        const startTime = performance.now();
        
        try {
            if (options.hardDelete) {
                // Hard delete - remove completely
                this.#users.delete(user.id);
                this.#userIndex.delete(user.email);
            } else {
                // Soft delete - mark as deleted
                const deletedUser = {
                    ...user,
                    isActive: false,
                    deletedAt: new Date(),
                    deletionReason: options.reason || 'No reason provided',
                    updatedAt: new Date()
                };
                this.#users.set(user.id, deletedUser);
            }
            
            // Clear caches
            this.clearCache(user.id);
            this.clearCache(user.email);
            
            // Update metrics
            this.#updateMetrics('delete', true, performance.now() - startTime);
            
            // Emit events
            this.emit('userDeleted', { 
                user, 
                hardDelete: options.hardDelete, 
                reason: options.reason 
            });
            
            console.log(`✅ Deleted user: ${user.email} (${options.hardDelete ? 'hard' : 'soft'})`);
            
            return true;
            
        } catch (error) {
            this.#updateMetrics('delete', false, performance.now() - startTime);
            console.error('❌ User deletion failed:', error);
            throw error;
        }
    }
    
    /**
     * Search users with advanced filtering and sorting
     * @async
     * @param {Object} [criteria={}] - Search criteria
     * @param {string} [criteria.query] - Text search query
     * @param {boolean} [criteria.isActive] - Filter by active status
     * @param {boolean} [criteria.isAdmin] - Filter by admin status
     * @param {Date} [criteria.createdAfter] - Filter by creation date
     * @param {QueryOptions} [options={}] - Query options
     * @returns {Promise<{users: UserData[], total: number, hasMore: boolean}>} Search results
     * @example
     * ```javascript
     * const results = await userManager.searchUsers({
     *   query: 'john',
     *   isActive: true
     * }, {
     *   limit: 10,
     *   sortBy: 'createdAt',
     *   sortOrder: 'desc'
     * });
     * ```
     */
    @performanceMonitor
    async searchUsers(criteria = {}, options = {}) {
        this.#ensureInitialized();
        
        const {
            limit = 50,
            offset = 0,
            sortBy = 'createdAt',
            sortOrder = 'desc'
        } = options;
        
        const startTime = performance.now();
        
        try {
            // Convert users map to array for filtering/sorting
            let users = Array.from(this.#users.values());
            
            // Apply filters
            users = users.filter(user => {
                // Text search
                if (criteria.query) {
                    const query = criteria.query.toLowerCase();
                    const searchFields = [
                        user.firstName,
                        user.lastName,
                        user.email,
                        user.profile?.bio || ''
                    ].join(' ').toLowerCase();
                    
                    if (!searchFields.includes(query)) {
                        return false;
                    }
                }
                
                // Active status filter
                if (criteria.isActive !== undefined && user.isActive !== criteria.isActive) {
                    return false;
                }
                
                // Admin status filter
                if (criteria.isAdmin !== undefined && user.isAdmin !== criteria.isAdmin) {
                    return false;
                }
                
                // Creation date filter
                if (criteria.createdAfter && user.createdAt < criteria.createdAfter) {
                    return false;
                }
                
                return true;
            });
            
            // Sort users
            users.sort((a, b) => {
                let aValue = a[sortBy];
                let bValue = b[sortBy];
                
                // Handle date fields
                if (aValue instanceof Date) {
                    aValue = aValue.getTime();
                    bValue = bValue.getTime();
                }
                
                // Handle string fields
                if (typeof aValue === 'string') {
                    aValue = aValue.toLowerCase();
                    bValue = bValue.toLowerCase();
                }
                
                const comparison = aValue < bValue ? -1 : aValue > bValue ? 1 : 0;
                return sortOrder === 'asc' ? comparison : -comparison;
            });
            
            // Apply pagination
            const total = users.length;
            const paginatedUsers = users.slice(offset, offset + limit);
            const hasMore = offset + limit < total;
            
            // Update metrics
            this.#updateMetrics('search', true, performance.now() - startTime);
            
            console.log(`🔍 Search returned ${paginatedUsers.length} of ${total} users`);
            
            return {
                users: paginatedUsers.map(user => ({ ...user })), // Return copies
                total,
                hasMore,
                query: criteria,
                options
            };
            
        } catch (error) {
            this.#updateMetrics('search', false, performance.now() - startTime);
            console.error('❌ User search failed:', error);
            throw error;
        }
    }
    
    /**
     * Perform batch operations on multiple users
     * @async
     * @param {string} operation - Operation type ('update', 'delete', 'activate', 'deactivate')
     * @param {string[]} userIds - Array of user IDs
     * @param {Object} [data={}] - Operation data (for updates)
     * @param {Object} [options={}] - Batch options
     * @param {number} [options.concurrency=5] - Maximum concurrent operations
     * @returns {Promise<{successful: string[], failed: {id: string, error: string}[]}>} Batch results
     * @example
     * ```javascript
     * const results = await userManager.batchOperation('activate', ['user1', 'user2', 'user3']);
     * ```
     */
    @performanceMonitor
    async batchOperation(operation, userIds, data = {}, options = {}) {
        this.#ensureInitialized();
        
        const { concurrency = 5 } = options;
        const results = { successful: [], failed: [] };
        
        console.log(`🔄 Starting batch ${operation} for ${userIds.length} users`);
        
        // Process users in batches to control concurrency
        for (let i = 0; i < userIds.length; i += concurrency) {
            const batch = userIds.slice(i, i + concurrency);
            
            const batchPromises = batch.map(async (userId) => {
                try {
                    switch (operation) {
                        case 'update':
                            await this.updateUser(userId, data, { skipValidation: true });
                            break;
                        case 'delete':
                            await this.deleteUser(userId, options);
                            break;
                        case 'activate':
                            await this.updateUser(userId, { isActive: true }, { skipValidation: true });
                            break;
                        case 'deactivate':
                            await this.updateUser(userId, { isActive: false }, { skipValidation: true });
                            break;
                        default:
                            throw new Error(`Unknown operation: ${operation}`);
                    }
                    
                    results.successful.push(userId);
                    
                } catch (error) {
                    results.failed.push({
                        id: userId,
                        error: error.message
                    });
                }
            });
            
            await Promise.all(batchPromises);
            
            // Small delay between batches to prevent overwhelming
            if (i + concurrency < userIds.length) {
                await new Promise(resolve => setTimeout(resolve, 10));
            }
        }
        
        console.log(`✅ Batch ${operation} completed: ${results.successful.length} successful, ${results.failed.length} failed`);
        
        return results;
    }
    
    /**
     * Export user data in various formats
     * @async
     * @param {string[]} [userIds] - Specific user IDs to export (all if not specified)
     * @param {Object} [options={}] - Export options
     * @param {('json'|'csv'|'xlsx')} [options.format='json'] - Export format
     * @param {boolean} [options.includeProfile=true] - Include profile data
     * @param {boolean} [options.includeMetadata=false] - Include system metadata
     * @returns {Promise<string|Buffer>} Exported data
     * @example
     * ```javascript
     * const jsonData = await userManager.exportUsers(['user1', 'user2'], {
     *   format: 'json',
     *   includeProfile: true
     * });
     * ```
     */
    @performanceMonitor
    async exportUsers(userIds, options = {}) {
        this.#ensureInitialized();
        
        const {
            format = 'json',
            includeProfile = true,
            includeMetadata = false
        } = options;
        
        // Get users to export
        let users;
        if (userIds && userIds.length > 0) {
            users = userIds.map(id => this.#getUserInternal(id)).filter(Boolean);
        } else {
            users = Array.from(this.#users.values());
        }
        
        // Filter fields based on options
        const exportData = users.map(user => {
            const userData = { ...user };
            
            if (!includeProfile) {
                delete userData.profile;
            }
            
            if (!includeMetadata) {
                delete userData.createdAt;
                delete userData.updatedAt;
                delete userData.deletedAt;
                delete userData.deletionReason;
            }
            
            return userData;
        });
        
        // Format data according to requested format
        switch (format) {
            case 'json':
                return JSON.stringify(exportData, null, 2);
                
            case 'csv':
                return this.#convertToCSV(exportData);
                
            case 'xlsx':
                // In a real implementation, this would use a library like xlsx
                throw new Error('XLSX format not implemented in this example');
                
            default:
                throw new Error(`Unsupported export format: ${format}`);
        }
    }
    
    /**
     * Clean up resources and perform graceful shutdown
     * @async
     * @returns {Promise<void>}
     */
    async destroy() {
        console.log('🔄 Shutting down UserManager...');
        
        // Clear backup interval
        if (this.#backupInterval) {
            clearInterval(this.#backupInterval);
            this.#backupInterval = null;
        }
        
        // Perform final backup
        if (this.#config.autoBackup) {
            await this.#performBackup();
        }
        
        // Clear all data
        this.#users.clear();
        this.#userIndex.clear();
        this.clearCache();
        
        // Remove all listeners
        this.removeAllListeners();
        
        this.#isInitialized = false;
        
        console.log('✅ UserManager shutdown complete');
    }
    
    // Private methods
    
    /**
     * Validate configuration object
     * @private
     * @throws {Error} When configuration is invalid
     */
    #validateConfig() {
        const { maxUsers, cacheTimeout } = this.#config;
        
        if (typeof maxUsers !== 'number' || maxUsers <= 0) {
            throw new Error('maxUsers must be a positive number');
        }
        
        if (typeof cacheTimeout !== 'number' || cacheTimeout < 0) {
            throw new Error('cacheTimeout must be a non-negative number');
        }
    }
    
    /**
     * Set up internal event listeners
     * @private
     */
    #setupEventListeners() {
        this.on('userCreated', (user) => {
            console.log(`📝 User created event: ${user.email}`);
        });
        
        this.on('userUpdated', ({ user, changes }) => {
            console.log(`📝 User updated event: ${user.email}`, Object.keys(changes));
        });
        
        this.on('userDeleted', ({ user, hardDelete }) => {
            console.log(`📝 User deleted event: ${user.email} (${hardDelete ? 'hard' : 'soft'})`);
        });
    }
    
    /**
     * Initialize validation rules
     * @private
     */
    #initializeValidators() {
        this.#validators.set('email', (email) => {
            const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
            return emailRegex.test(email);
        });
        
        this.#validators.set('name', (name) => {
            return typeof name === 'string' && name.trim().length >= 1 && name.length <= 50;
        });
        
        this.#validators.set('phone', (phone) => {
            const phoneRegex = /^\+?[\d\s\-\(\)]+$/;
            return !phone || phoneRegex.test(phone);
        });
    }
    
    /**
     * Ensure manager is initialized
     * @private
     * @throws {Error} When not initialized
     */
    #ensureInitialized() {
        if (!this.#isInitialized) {
            throw new Error('UserManager not initialized. Call initialize() first.');
        }
    }
    
    /**
     * Generate unique user ID
     * @private
     * @returns {string} Unique user ID
     */
    #generateUserId() {
        return `user_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }
    
    /**
     * Internal user retrieval method
     * @private
     * @param {string} identifier - User ID or email
     * @returns {UserData|null} User object or null
     */
    #getUserInternal(identifier) {
        // Try as direct ID first
        let user = this.#users.get(identifier);
        
        if (!user) {
            // Try as email
            const userId = this.#userIndex.get(identifier.toLowerCase());
            if (userId) {
                user = this.#users.get(userId);
            }
        }
        
        return user ? { ...user } : null;
    }
    
    /**
     * Validate user data
     * @private
     * @param {Object} userData - User data to validate
     * @param {boolean} [isUpdate=false] - Whether this is an update operation
     * @returns {Promise<ValidationResult>} Validation result
     */
    async #validateUserData(userData, isUpdate = false) {
        const errors = [];
        const processedData = { ...userData };
        
        // Email validation
        if (!isUpdate || userData.email !== undefined) {
            if (!userData.email) {
                errors.push('Email is required');
            } else if (!this.#validators.get('email')(userData.email)) {
                errors.push('Invalid email format');
            } else {
                processedData.email = userData.email.toLowerCase().trim();
            }
        }
        
        // Name validation
        if (!isUpdate || userData.firstName !== undefined) {
            if (!this.#validators.get('name')(userData.firstName)) {
                errors.push('First name must be 1-50 characters');
            } else {
                processedData.firstName = userData.firstName.trim();
            }
        }
        
        if (!isUpdate || userData.lastName !== undefined) {
            if (!this.#validators.get('name')(userData.lastName)) {
                errors.push('Last name must be 1-50 characters');
            } else {
                processedData.lastName = userData.lastName.trim();
            }
        }
        
        // Profile validation
        if (userData.profile) {
            if (userData.profile.phone && !this.#validators.get('phone')(userData.profile.phone)) {
                errors.push('Invalid phone number format');
            }
        }
        
        return {
            isValid: errors.length === 0,
            errors,
            data: processedData
        };
    }
    
    /**
     * Update performance metrics
     * @private
     * @param {string} operation - Operation name
     * @param {boolean} success - Whether operation succeeded
     * @param {number} duration - Operation duration in ms
     */
    #updateMetrics(operation, success, duration) {
        if (!this.#config.enableMetrics) return;
        
        this.#metrics.totalOperations++;
        
        if (success) {
            this.#metrics.successfulOperations++;
        } else {
            this.#metrics.failedOperations++;
        }
        
        // Update average response time
        const total = this.#metrics.totalOperations;
        this.#metrics.averageResponseTime = 
            (this.#metrics.averageResponseTime * (total - 1) + duration) / total;
    }
    
    /**
     * Simulate async operation
     * @private
     * @param {string} operation - Operation name
     * @param {number} delay - Delay in milliseconds
     * @returns {Promise<void>}
     */
    async #simulateAsyncOperation(operation, delay) {
        return new Promise((resolve) => {
            setTimeout(() => {
                console.log(`⏱️  Completed ${operation} (${delay}ms)`);
                resolve();
            }, delay);
        });
    }
    
    /**
     * Send welcome email (simulated)
     * @private
     * @param {UserData} user - User object
     * @returns {Promise<void>}
     */
    async #sendWelcomeEmail(user) {
        await this.#simulateAsyncOperation(`welcome email to ${user.email}`, 50);
    }
    
    /**
     * Create user snapshot for backup
     * @private
     * @param {UserData} user - User object
     * @returns {Promise<void>}
     */
    async #createUserSnapshot(user) {
        // In a real implementation, this would save to a backup system
        console.log(`📸 Created snapshot for user ${user.id}`);
    }
    
    /**
     * Perform backup operation
     * @private
     * @returns {Promise<void>}
     */
    async #performBackup() {
        if (this.userCount === 0) return;
        
        console.log(`💾 Performing backup of ${this.userCount} users`);
        
        // In a real implementation, this would save to persistent storage
        const backupData = {
            timestamp: new Date().toISOString(),
            userCount: this.userCount,
            metrics: this.metrics,
            users: Array.from(this.#users.values())
        };
        
        console.log('✅ Backup completed');
    }
    
    /**
     * Convert data to CSV format
     * @private
     * @param {Array} data - Data to convert
     * @returns {string} CSV string
     */
    #convertToCSV(data) {
        if (data.length === 0) return '';
        
        const headers = Object.keys(data[0]);
        const csvRows = [headers.join(',')];
        
        for (const row of data) {
            const values = headers.map(header => {
                const value = row[header];
                return typeof value === 'string' ? `"${value.replace(/"/g, '""')}"` : value;
            });
            csvRows.push(values.join(','));
        }
        
        return csvRows.join('\n');
    }
    
    // Static utility methods
    
    /**
     * Validate email format
     * @static
     * @param {string} email - Email to validate
     * @returns {boolean} Whether email is valid
     */
    static validateEmail(email) {
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return emailRegex.test(email);
    }
    
    /**
     * Generate a random password
     * @static
     * @param {number} [length=12] - Password length
     * @returns {string} Generated password
     */
    static generatePassword(length = 12) {
        const charset = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*';
        let password = '';
        
        for (let i = 0; i < length; i++) {
            password += charset.charAt(Math.floor(Math.random() * charset.length));
        }
        
        return password;
    }
    
    /**
     * Get version information
     * @static
     * @returns {Object} Version and build information
     */
    static getVersionInfo() {
        return {
            version: AdvancedUserManager.VERSION,
            supportedOperations: AdvancedUserManager.SUPPORTED_OPERATIONS,
            buildDate: new Date().toISOString()
        };
    }
}

// Export the class and related utilities
export default AdvancedUserManager;

export {
    AdvancedUserManager,
    ObservableMixin,
    CacheableMixin,
    performanceMonitor,
    validate,
    cache
};

/**
 * @namespace UserManagerUtils
 * @description Utility functions for user management
 */
export const UserManagerUtils = {
    /**
     * Format user display name
     * @param {UserData} user - User object
     * @returns {string} Formatted display name
     */
    formatDisplayName(user) {
        return `${user.firstName} ${user.lastName}`.trim() || user.email;
    },
    
    /**
     * Check if user is recently active
     * @param {UserData} user - User object
     * @param {number} [daysThreshold=30] - Days to consider as recent
     * @returns {boolean} Whether user is recently active
     */
    isRecentlyActive(user, daysThreshold = 30) {
        if (!user.lastLoginAt) return false;
        
        const now = new Date();
        const lastLogin = new Date(user.lastLoginAt);
        const daysDiff = (now - lastLogin) / (1000 * 60 * 60 * 24);
        
        return daysDiff <= daysThreshold;
    },
    
    /**
     * Get user age in days
     * @param {UserData} user - User object
     * @returns {number} Age in days
     */
    getUserAgeDays(user) {
        const now = new Date();
        const created = new Date(user.createdAt);
        return Math.floor((now - created) / (1000 * 60 * 60 * 24));
    }
};