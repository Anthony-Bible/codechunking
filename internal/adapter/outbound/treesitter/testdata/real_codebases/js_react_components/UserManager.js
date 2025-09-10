import React, { useState, useEffect, useCallback, useMemo } from 'react';
import PropTypes from 'prop-types';
import './UserManager.css';

/**
 * UserManager component for handling user CRUD operations
 */
class UserManager extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            users: [],
            loading: false,
            error: null,
            selectedUser: null,
            showModal: false,
        };

        this.apiClient = new APIClient(props.apiBaseUrl);
    }

    componentDidMount() {
        this.loadUsers();
    }

    componentDidUpdate(prevProps) {
        if (prevProps.apiBaseUrl !== this.props.apiBaseUrl) {
            this.apiClient = new APIClient(this.props.apiBaseUrl);
            this.loadUsers();
        }
    }

    async loadUsers() {
        try {
            this.setState({ loading: true, error: null });
            const users = await this.apiClient.getUsers();
            this.setState({ users });
        } catch (error) {
            console.error('Failed to load users:', error);
            this.setState({ error: error.message });
        } finally {
            this.setState({ loading: false });
        }
    }

    handleUserSelect = (user) => {
        this.setState({ selectedUser: user, showModal: true });
    };

    handleUserCreate = async (userData) => {
        try {
            const newUser = await this.apiClient.createUser(userData);
            this.setState(prevState => ({
                users: [...prevState.users, newUser],
                showModal: false,
            }));
        } catch (error) {
            console.error('Failed to create user:', error);
            this.setState({ error: error.message });
        }
    };

    handleUserUpdate = async (userId, userData) => {
        try {
            const updatedUser = await this.apiClient.updateUser(userId, userData);
            this.setState(prevState => ({
                users: prevState.users.map(user => 
                    user.id === userId ? updatedUser : user
                ),
                showModal: false,
                selectedUser: null,
            }));
        } catch (error) {
            console.error('Failed to update user:', error);
            this.setState({ error: error.message });
        }
    };

    handleUserDelete = async (userId) => {
        if (!window.confirm('Are you sure you want to delete this user?')) {
            return;
        }

        try {
            await this.apiClient.deleteUser(userId);
            this.setState(prevState => ({
                users: prevState.users.filter(user => user.id !== userId),
                selectedUser: null,
                showModal: false,
            }));
        } catch (error) {
            console.error('Failed to delete user:', error);
            this.setState({ error: error.message });
        }
    };

    render() {
        const { users, loading, error, selectedUser, showModal } = this.state;

        if (loading) {
            return <LoadingSpinner />;
        }

        if (error) {
            return (
                <ErrorDisplay 
                    error={error} 
                    onRetry={() => this.loadUsers()} 
                />
            );
        }

        return (
            <div className="user-manager">
                <div className="user-manager-header">
                    <h2>User Management</h2>
                    <button 
                        className="btn btn-primary"
                        onClick={() => this.setState({ showModal: true, selectedUser: null })}
                    >
                        Add New User
                    </button>
                </div>

                <UserList 
                    users={users}
                    onUserSelect={this.handleUserSelect}
                    onUserDelete={this.handleUserDelete}
                />

                {showModal && (
                    <UserModal
                        user={selectedUser}
                        onSave={selectedUser ? 
                            (data) => this.handleUserUpdate(selectedUser.id, data) :
                            this.handleUserCreate
                        }
                        onClose={() => this.setState({ showModal: false, selectedUser: null })}
                    />
                )}
            </div>
        );
    }
}

UserManager.propTypes = {
    apiBaseUrl: PropTypes.string.isRequired,
};

UserManager.defaultProps = {
    apiBaseUrl: 'http://localhost:3000/api',
};

// Functional components for UI elements

const LoadingSpinner = () => (
    <div className="loading-spinner">
        <div className="spinner" />
        <p>Loading users...</p>
    </div>
);

const ErrorDisplay = ({ error, onRetry }) => (
    <div className="error-display">
        <h3>Error Loading Users</h3>
        <p>{error}</p>
        <button className="btn btn-secondary" onClick={onRetry}>
            Retry
        </button>
    </div>
);

ErrorDisplay.propTypes = {
    error: PropTypes.string.isRequired,
    onRetry: PropTypes.func.isRequired,
};

const UserList = ({ users, onUserSelect, onUserDelete }) => {
    const sortedUsers = useMemo(() => 
        [...users].sort((a, b) => a.name.localeCompare(b.name)),
        [users]
    );

    if (users.length === 0) {
        return (
            <div className="empty-state">
                <p>No users found. Create your first user to get started.</p>
            </div>
        );
    }

    return (
        <div className="user-list">
            {sortedUsers.map(user => (
                <UserCard
                    key={user.id}
                    user={user}
                    onSelect={() => onUserSelect(user)}
                    onDelete={() => onUserDelete(user.id)}
                />
            ))}
        </div>
    );
};

UserList.propTypes = {
    users: PropTypes.arrayOf(PropTypes.shape({
        id: PropTypes.number.isRequired,
        name: PropTypes.string.isRequired,
        email: PropTypes.string.isRequired,
    })).isRequired,
    onUserSelect: PropTypes.func.isRequired,
    onUserDelete: PropTypes.func.isRequired,
};

const UserCard = ({ user, onSelect, onDelete }) => (
    <div className="user-card">
        <div className="user-info" onClick={onSelect}>
            <h4>{user.name}</h4>
            <p>{user.email}</p>
            <small>ID: {user.id}</small>
        </div>
        <div className="user-actions">
            <button 
                className="btn btn-sm btn-outline"
                onClick={onSelect}
            >
                Edit
            </button>
            <button 
                className="btn btn-sm btn-danger"
                onClick={onDelete}
            >
                Delete
            </button>
        </div>
    </div>
);

UserCard.propTypes = {
    user: PropTypes.shape({
        id: PropTypes.number.isRequired,
        name: PropTypes.string.isRequired,
        email: PropTypes.string.isRequired,
    }).isRequired,
    onSelect: PropTypes.func.isRequired,
    onDelete: PropTypes.func.isRequired,
};

const UserModal = ({ user, onSave, onClose }) => {
    const [formData, setFormData] = useState({
        name: user?.name || '',
        email: user?.email || '',
    });
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setIsSubmitting(true);
        
        try {
            await onSave(formData);
        } catch (error) {
            console.error('Error saving user:', error);
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleChange = useCallback((field, value) => {
        setFormData(prev => ({
            ...prev,
            [field]: value,
        }));
    }, []);

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal-content" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h3>{user ? 'Edit User' : 'Create New User'}</h3>
                    <button className="modal-close" onClick={onClose}>×</button>
                </div>
                
                <form onSubmit={handleSubmit} className="user-form">
                    <div className="form-group">
                        <label htmlFor="name">Name</label>
                        <input
                            id="name"
                            type="text"
                            value={formData.name}
                            onChange={(e) => handleChange('name', e.target.value)}
                            required
                            disabled={isSubmitting}
                        />
                    </div>
                    
                    <div className="form-group">
                        <label htmlFor="email">Email</label>
                        <input
                            id="email"
                            type="email"
                            value={formData.email}
                            onChange={(e) => handleChange('email', e.target.value)}
                            required
                            disabled={isSubmitting}
                        />
                    </div>
                    
                    <div className="form-actions">
                        <button 
                            type="button" 
                            className="btn btn-secondary"
                            onClick={onClose}
                            disabled={isSubmitting}
                        >
                            Cancel
                        </button>
                        <button 
                            type="submit" 
                            className="btn btn-primary"
                            disabled={isSubmitting}
                        >
                            {isSubmitting ? 'Saving...' : 'Save'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
};

UserModal.propTypes = {
    user: PropTypes.shape({
        id: PropTypes.number,
        name: PropTypes.string,
        email: PropTypes.string,
    }),
    onSave: PropTypes.func.isRequired,
    onClose: PropTypes.func.isRequired,
};

// API Client helper class
class APIClient {
    constructor(baseUrl) {
        this.baseUrl = baseUrl;
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseUrl}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers,
            },
            ...options,
        };

        if (config.body && typeof config.body === 'object') {
            config.body = JSON.stringify(config.body);
        }

        const response = await fetch(url, config);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        return response.json();
    }

    async getUsers() {
        const result = await this.request('/users');
        return result.users || [];
    }

    async createUser(userData) {
        const result = await this.request('/users', {
            method: 'POST',
            body: userData,
        });
        return result.user;
    }

    async updateUser(userId, userData) {
        const result = await this.request(`/users/${userId}`, {
            method: 'PUT',
            body: userData,
        });
        return result.user;
    }

    async deleteUser(userId) {
        await this.request(`/users/${userId}`, {
            method: 'DELETE',
        });
    }
}

export default UserManager;