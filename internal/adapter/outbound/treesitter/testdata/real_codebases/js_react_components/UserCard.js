/**
 * UserCard component for displaying user information.
 * This component handles user avatar, name, email, and status display
 * with interactive features like editing and actions.
 */

import React, { useState, useEffect, useCallback, useMemo } from 'react';
import PropTypes from 'prop-types';
import classNames from 'classnames';
import { Avatar, Badge, Button, Dropdown, Menu, Modal, message } from 'antd';
import { 
    UserOutlined, 
    EditOutlined, 
    DeleteOutlined, 
    MoreOutlined,
    MailOutlined,
    PhoneOutlined,
    GlobalOutlined
} from '@ant-design/icons';

import './UserCard.scss';

/**
 * User data shape for TypeScript-like prop validation
 */
const UserShape = PropTypes.shape({
    id: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
    email: PropTypes.string.isRequired,
    firstName: PropTypes.string.isRequired,
    lastName: PropTypes.string.isRequired,
    avatarUrl: PropTypes.string,
    isActive: PropTypes.bool,
    isAdmin: PropTypes.bool,
    bio: PropTypes.string,
    location: PropTypes.string,
    website: PropTypes.string,
    phone: PropTypes.string,
    joinedAt: PropTypes.string,
    lastLoginAt: PropTypes.string,
});

/**
 * UserCard component - displays a user's information in a card format
 * 
 * @param {Object} props - Component properties
 * @param {Object} props.user - User data object
 * @param {boolean} props.showActions - Whether to show action buttons
 * @param {boolean} props.isCompact - Whether to use compact layout
 * @param {boolean} props.isSelectable - Whether the card is selectable
 * @param {boolean} props.isSelected - Whether the card is currently selected
 * @param {Function} props.onEdit - Callback when edit button is clicked
 * @param {Function} props.onDelete - Callback when delete button is clicked
 * @param {Function} props.onSelect - Callback when card is selected
 * @param {Function} props.onUserClick - Callback when user info is clicked
 * @param {string} props.className - Additional CSS classes
 * @param {Object} props.style - Inline styles
 * @returns {JSX.Element} UserCard component
 */
const UserCard = ({
    user,
    showActions = true,
    isCompact = false,
    isSelectable = false,
    isSelected = false,
    onEdit,
    onDelete,
    onSelect,
    onUserClick,
    className,
    style,
    ...restProps
}) => {
    // State management
    const [isHovered, setIsHovered] = useState(false);
    const [isExpanded, setIsExpanded] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [deleteModalVisible, setDeleteModalVisible] = useState(false);

    // Memoized computed values
    const fullName = useMemo(() => {
        return `${user.firstName} ${user.lastName}`.trim();
    }, [user.firstName, user.lastName]);

    const initials = useMemo(() => {
        const first = user.firstName?.[0]?.toUpperCase() || '';
        const last = user.lastName?.[0]?.toUpperCase() || '';
        return `${first}${last}`;
    }, [user.firstName, user.lastName]);

    const statusBadge = useMemo(() => {
        if (user.isAdmin) {
            return { status: 'processing', text: 'Admin' };
        }
        if (user.isActive) {
            return { status: 'success', text: 'Active' };
        }
        return { status: 'default', text: 'Inactive' };
    }, [user.isActive, user.isAdmin]);

    const formattedJoinDate = useMemo(() => {
        if (!user.joinedAt) return null;
        try {
            return new Date(user.joinedAt).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            });
        } catch {
            return user.joinedAt;
        }
    }, [user.joinedAt]);

    const formattedLastLogin = useMemo(() => {
        if (!user.lastLoginAt) return 'Never';
        try {
            const date = new Date(user.lastLoginAt);
            const now = new Date();
            const diffInHours = (now - date) / (1000 * 60 * 60);
            
            if (diffInHours < 1) {
                return 'Just now';
            } else if (diffInHours < 24) {
                return `${Math.floor(diffInHours)} hours ago`;
            } else if (diffInHours < 48) {
                return 'Yesterday';
            } else {
                return date.toLocaleDateString('en-US', {
                    month: 'short',
                    day: 'numeric'
                });
            }
        } catch {
            return user.lastLoginAt;
        }
    }, [user.lastLoginAt]);

    // Event handlers
    const handleMouseEnter = useCallback(() => {
        setIsHovered(true);
    }, []);

    const handleMouseLeave = useCallback(() => {
        setIsHovered(false);
    }, []);

    const handleCardClick = useCallback((event) => {
        if (event.target.closest('.user-card__actions')) {
            return; // Don't trigger card click if clicking on actions
        }

        if (isSelectable && onSelect) {
            onSelect(user, !isSelected);
        } else if (onUserClick) {
            onUserClick(user);
        }
    }, [user, isSelectable, isSelected, onSelect, onUserClick]);

    const handleEditClick = useCallback((event) => {
        event.stopPropagation();
        if (onEdit) {
            onEdit(user);
        }
    }, [user, onEdit]);

    const handleDeleteClick = useCallback((event) => {
        event.stopPropagation();
        setDeleteModalVisible(true);
    }, []);

    const confirmDelete = useCallback(async () => {
        if (onDelete) {
            setIsDeleting(true);
            try {
                await onDelete(user);
                message.success(`User ${fullName} deleted successfully`);
            } catch (error) {
                message.error(`Failed to delete user: ${error.message}`);
            } finally {
                setIsDeleting(false);
                setDeleteModalVisible(false);
            }
        }
    }, [user, onDelete, fullName]);

    const cancelDelete = useCallback(() => {
        setDeleteModalVisible(false);
    }, []);

    const toggleExpanded = useCallback((event) => {
        event.stopPropagation();
        setIsExpanded(!isExpanded);
    }, [isExpanded]);

    // Action menu items
    const actionMenuItems = useMemo(() => {
        const items = [
            {
                key: 'edit',
                icon: <EditOutlined />,
                label: 'Edit User',
                onClick: handleEditClick,
            },
            {
                key: 'delete',
                icon: <DeleteOutlined />,
                label: 'Delete User',
                onClick: handleDeleteClick,
                danger: true,
            },
        ];

        if (!isCompact) {
            items.unshift({
                key: 'expand',
                icon: isExpanded ? <UserOutlined /> : <MoreOutlined />,
                label: isExpanded ? 'Show Less' : 'Show More',
                onClick: toggleExpanded,
            });
        }

        return { items };
    }, [handleEditClick, handleDeleteClick, toggleExpanded, isExpanded, isCompact]);

    // CSS classes
    const cardClasses = classNames(
        'user-card',
        {
            'user-card--compact': isCompact,
            'user-card--selectable': isSelectable,
            'user-card--selected': isSelected,
            'user-card--hovered': isHovered,
            'user-card--expanded': isExpanded,
            'user-card--inactive': !user.isActive,
        },
        className
    );

    // Render contact information
    const renderContactInfo = () => {
        if (isCompact) return null;

        return (
            <div className="user-card__contact">
                {user.email && (
                    <div className="user-card__contact-item">
                        <MailOutlined className="user-card__contact-icon" />
                        <a href={`mailto:${user.email}`} className="user-card__contact-link">
                            {user.email}
                        </a>
                    </div>
                )}
                {user.phone && (
                    <div className="user-card__contact-item">
                        <PhoneOutlined className="user-card__contact-icon" />
                        <a href={`tel:${user.phone}`} className="user-card__contact-link">
                            {user.phone}
                        </a>
                    </div>
                )}
                {user.website && (
                    <div className="user-card__contact-item">
                        <GlobalOutlined className="user-card__contact-icon" />
                        <a 
                            href={user.website} 
                            target="_blank" 
                            rel="noopener noreferrer"
                            className="user-card__contact-link"
                        >
                            {user.website.replace(/^https?:\/\//, '')}
                        </a>
                    </div>
                )}
            </div>
        );
    };

    // Render expanded details
    const renderExpandedDetails = () => {
        if (!isExpanded || isCompact) return null;

        return (
            <div className="user-card__details">
                {user.bio && (
                    <div className="user-card__bio">
                        <h4>Bio</h4>
                        <p>{user.bio}</p>
                    </div>
                )}
                <div className="user-card__meta">
                    {user.location && (
                        <div className="user-card__meta-item">
                            <strong>Location:</strong> {user.location}
                        </div>
                    )}
                    {formattedJoinDate && (
                        <div className="user-card__meta-item">
                            <strong>Joined:</strong> {formattedJoinDate}
                        </div>
                    )}
                    <div className="user-card__meta-item">
                        <strong>Last Login:</strong> {formattedLastLogin}
                    </div>
                </div>
            </div>
        );
    };

    // Render actions
    const renderActions = () => {
        if (!showActions) return null;

        if (isCompact) {
            return (
                <div className="user-card__actions user-card__actions--compact">
                    <Dropdown menu={actionMenuItems} trigger={['click']} placement="bottomRight">
                        <Button
                            type="text"
                            icon={<MoreOutlined />}
                            size="small"
                            className="user-card__action-button"
                            onClick={(e) => e.stopPropagation()}
                        />
                    </Dropdown>
                </div>
            );
        }

        return (
            <div className="user-card__actions">
                <Button
                    type="text"
                    icon={<EditOutlined />}
                    size="small"
                    onClick={handleEditClick}
                    className="user-card__action-button"
                >
                    Edit
                </Button>
                <Button
                    type="text"
                    icon={<DeleteOutlined />}
                    size="small"
                    onClick={handleDeleteClick}
                    danger
                    className="user-card__action-button"
                >
                    Delete
                </Button>
            </div>
        );
    };

    return (
        <>
            <div
                className={cardClasses}
                style={style}
                onClick={handleCardClick}
                onMouseEnter={handleMouseEnter}
                onMouseLeave={handleMouseLeave}
                {...restProps}
            >
                <div className="user-card__header">
                    <div className="user-card__avatar-section">
                        <Avatar
                            size={isCompact ? 40 : 60}
                            src={user.avatarUrl}
                            icon={<UserOutlined />}
                            className="user-card__avatar"
                        >
                            {!user.avatarUrl && initials}
                        </Avatar>
                        {!isCompact && (
                            <Badge
                                status={statusBadge.status}
                                text={statusBadge.text}
                                className="user-card__badge"
                            />
                        )}
                    </div>

                    <div className="user-card__info">
                        <div className="user-card__name-section">
                            <h3 className="user-card__name">
                                {fullName}
                                {isCompact && (
                                    <Badge
                                        status={statusBadge.status}
                                        className="user-card__badge--inline"
                                    />
                                )}
                            </h3>
                            {!isCompact && (
                                <span className="user-card__email">{user.email}</span>
                            )}
                        </div>
                        {renderActions()}
                    </div>
                </div>

                {renderContactInfo()}
                {renderExpandedDetails()}

                {isSelectable && (
                    <div className="user-card__selection-indicator">
                        {isSelected && <div className="user-card__selection-checkmark">✓</div>}
                    </div>
                )}
            </div>

            <Modal
                title="Confirm Delete"
                open={deleteModalVisible}
                onOk={confirmDelete}
                onCancel={cancelDelete}
                confirmLoading={isDeleting}
                okType="danger"
                okText="Delete"
                cancelText="Cancel"
            >
                <p>Are you sure you want to delete user <strong>{fullName}</strong>?</p>
                <p>This action cannot be undone.</p>
            </Modal>
        </>
    );
};

// PropTypes validation
UserCard.propTypes = {
    user: UserShape.isRequired,
    showActions: PropTypes.bool,
    isCompact: PropTypes.bool,
    isSelectable: PropTypes.bool,
    isSelected: PropTypes.bool,
    onEdit: PropTypes.func,
    onDelete: PropTypes.func,
    onSelect: PropTypes.func,
    onUserClick: PropTypes.func,
    className: PropTypes.string,
    style: PropTypes.object,
};

// Default props
UserCard.defaultProps = {
    showActions: true,
    isCompact: false,
    isSelectable: false,
    isSelected: false,
    onEdit: undefined,
    onDelete: undefined,
    onSelect: undefined,
    onUserClick: undefined,
    className: '',
    style: {},
};

export default UserCard;

// Named exports for convenience
export { UserShape };

// Higher-order component for memoization
export const MemoizedUserCard = React.memo(UserCard, (prevProps, nextProps) => {
    // Custom comparison for optimization
    const userChanged = JSON.stringify(prevProps.user) !== JSON.stringify(nextProps.user);
    const propsChanged = 
        prevProps.isSelected !== nextProps.isSelected ||
        prevProps.showActions !== nextProps.showActions ||
        prevProps.isCompact !== nextProps.isCompact ||
        prevProps.isSelectable !== nextProps.isSelectable ||
        prevProps.className !== nextProps.className;
    
    return !userChanged && !propsChanged;
});

// Hook for user card functionality
export const useUserCard = (initialUser) => {
    const [user, setUser] = useState(initialUser);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState(null);

    const updateUser = useCallback(async (updates) => {
        setIsLoading(true);
        setError(null);
        try {
            // Simulate API call
            const updatedUser = { ...user, ...updates };
            setUser(updatedUser);
            return updatedUser;
        } catch (err) {
            setError(err);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [user]);

    const deleteUser = useCallback(async () => {
        setIsLoading(true);
        setError(null);
        try {
            // Simulate API call
            await new Promise(resolve => setTimeout(resolve, 1000));
            return true;
        } catch (err) {
            setError(err);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, []);

    return {
        user,
        isLoading,
        error,
        updateUser,
        deleteUser,
        setUser,
    };
};