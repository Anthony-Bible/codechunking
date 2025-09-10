/**
 * UserList component for displaying and managing a list of users.
 * Features include filtering, sorting, pagination, bulk actions, and real-time updates.
 */

import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { 
    Table, 
    Input, 
    Button, 
    Select, 
    Pagination, 
    Checkbox, 
    Space, 
    message, 
    Modal,
    Dropdown,
    Tag,
    Tooltip,
    Empty,
    Spin
} from 'antd';
import {
    SearchOutlined,
    FilterOutlined,
    UserAddOutlined,
    DeleteOutlined,
    DownloadOutlined,
    ReloadOutlined,
    SettingOutlined,
    ExclamationCircleOutlined
} from '@ant-design/icons';

import UserCard, { UserShape } from './UserCard';
import { useDebounce } from '../hooks/useDebounce';
import { useLocalStorage } from '../hooks/useLocalStorage';
import UserService from '../services/UserService';
import './UserList.scss';

const { Search } = Input;
const { Option } = Select;
const { confirm } = Modal;

/**
 * UserList component - displays and manages a paginated list of users
 * 
 * @param {Object} props - Component properties
 * @param {Array} props.users - Array of user objects
 * @param {boolean} props.loading - Whether the list is loading
 * @param {number} props.total - Total number of users
 * @param {number} props.page - Current page number
 * @param {number} props.pageSize - Number of items per page
 * @param {string} props.viewMode - Display mode: 'table' or 'cards'
 * @param {boolean} props.showBulkActions - Whether to show bulk action controls
 * @param {Function} props.onPageChange - Callback when page changes
 * @param {Function} props.onPageSizeChange - Callback when page size changes
 * @param {Function} props.onSearch - Callback when search query changes
 * @param {Function} props.onFilter - Callback when filters change
 * @param {Function} props.onSort - Callback when sorting changes
 * @param {Function} props.onUserEdit - Callback when a user is edited
 * @param {Function} props.onUserDelete - Callback when a user is deleted
 * @param {Function} props.onBulkAction - Callback for bulk actions
 * @param {Function} props.onUserAdd - Callback to add a new user
 * @param {Function} props.onRefresh - Callback to refresh the list
 */
const UserList = ({
    users = [],
    loading = false,
    total = 0,
    page = 1,
    pageSize = 20,
    viewMode: initialViewMode = 'table',
    showBulkActions = true,
    onPageChange,
    onPageSizeChange,
    onSearch,
    onFilter,
    onSort,
    onUserEdit,
    onUserDelete,
    onBulkAction,
    onUserAdd,
    onRefresh,
}) => {
    // State management
    const [selectedUsers, setSelectedUsers] = useState(new Set());
    const [searchQuery, setSearchQuery] = useState('');
    const [filters, setFilters] = useState({});
    const [sortField, setSortField] = useState('name');
    const [sortOrder, setSortOrder] = useState('ascend');
    const [viewMode, setViewMode] = useLocalStorage('userListViewMode', initialViewMode);
    const [columnSettings, setColumnSettings] = useLocalStorage('userListColumns', {
        name: true,
        email: true,
        status: true,
        role: true,
        lastLogin: true,
        actions: true,
    });

    // Refs
    const searchInputRef = useRef(null);
    const listContainerRef = useRef(null);

    // Debounced search
    const debouncedSearchQuery = useDebounce(searchQuery, 300);

    // Memoized values
    const selectedUserCount = selectedUsers.size;
    const allUsersSelected = selectedUserCount === users.length && users.length > 0;
    const someUsersSelected = selectedUserCount > 0 && selectedUserCount < users.length;

    const filteredAndSortedUsers = useMemo(() => {
        let result = [...users];

        // Apply search filter
        if (debouncedSearchQuery) {
            const query = debouncedSearchQuery.toLowerCase();
            result = result.filter(user => 
                user.firstName?.toLowerCase().includes(query) ||
                user.lastName?.toLowerCase().includes(query) ||
                user.email?.toLowerCase().includes(query)
            );
        }

        // Apply filters
        Object.entries(filters).forEach(([key, value]) => {
            if (value && value !== 'all') {
                result = result.filter(user => {
                    switch (key) {
                        case 'status':
                            return user.isActive === (value === 'active');
                        case 'role':
                            return user.isAdmin === (value === 'admin');
                        default:
                            return user[key] === value;
                    }
                });
            }
        });

        // Apply sorting
        result.sort((a, b) => {
            let aValue, bValue;
            
            switch (sortField) {
                case 'name':
                    aValue = `${a.firstName} ${a.lastName}`.toLowerCase();
                    bValue = `${b.firstName} ${b.lastName}`.toLowerCase();
                    break;
                case 'email':
                    aValue = a.email?.toLowerCase() || '';
                    bValue = b.email?.toLowerCase() || '';
                    break;
                case 'lastLogin':
                    aValue = new Date(a.lastLoginAt || 0);
                    bValue = new Date(b.lastLoginAt || 0);
                    break;
                default:
                    aValue = a[sortField] || '';
                    bValue = b[sortField] || '';
            }

            if (aValue < bValue) return sortOrder === 'ascend' ? -1 : 1;
            if (aValue > bValue) return sortOrder === 'ascend' ? 1 : -1;
            return 0;
        });

        return result;
    }, [users, debouncedSearchQuery, filters, sortField, sortOrder]);

    // Effects
    useEffect(() => {
        if (onSearch) {
            onSearch(debouncedSearchQuery);
        }
    }, [debouncedSearchQuery, onSearch]);

    useEffect(() => {
        if (onFilter) {
            onFilter(filters);
        }
    }, [filters, onFilter]);

    useEffect(() => {
        if (onSort) {
            onSort({ field: sortField, order: sortOrder });
        }
    }, [sortField, sortOrder, onSort]);

    // Event handlers
    const handleSelectAll = useCallback((checked) => {
        if (checked) {
            setSelectedUsers(new Set(users.map(user => user.id)));
        } else {
            setSelectedUsers(new Set());
        }
    }, [users]);

    const handleSelectUser = useCallback((userId, checked) => {
        const newSelected = new Set(selectedUsers);
        if (checked) {
            newSelected.add(userId);
        } else {
            newSelected.delete(userId);
        }
        setSelectedUsers(newSelected);
    }, [selectedUsers]);

    const handleSearchChange = useCallback((event) => {
        setSearchQuery(event.target.value);
    }, []);

    const handleFilterChange = useCallback((key, value) => {
        setFilters(prev => ({
            ...prev,
            [key]: value
        }));
    }, []);

    const handleSortChange = useCallback((sorter) => {
        if (sorter.field) {
            setSortField(sorter.field);
            setSortOrder(sorter.order);
        }
    }, []);

    const handleBulkDelete = useCallback(() => {
        if (selectedUserCount === 0) return;

        confirm({
            title: 'Confirm Bulk Delete',
            icon: <ExclamationCircleOutlined />,
            content: `Are you sure you want to delete ${selectedUserCount} selected user(s)?`,
            onOk: async () => {
                try {
                    if (onBulkAction) {
                        await onBulkAction('delete', Array.from(selectedUsers));
                        setSelectedUsers(new Set());
                        message.success(`Successfully deleted ${selectedUserCount} user(s)`);
                    }
                } catch (error) {
                    message.error(`Failed to delete users: ${error.message}`);
                }
            },
        });
    }, [selectedUserCount, selectedUsers, onBulkAction]);

    const handleExport = useCallback(async () => {
        try {
            const usersToExport = selectedUserCount > 0 
                ? users.filter(user => selectedUsers.has(user.id))
                : users;
            
            await UserService.exportUsers(usersToExport);
            message.success('Users exported successfully');
        } catch (error) {
            message.error(`Export failed: ${error.message}`);
        }
    }, [users, selectedUsers, selectedUserCount]);

    const handleRefresh = useCallback(() => {
        setSelectedUsers(new Set());
        if (onRefresh) {
            onRefresh();
        }
    }, [onRefresh]);

    const handleColumnSettingsChange = useCallback((column, visible) => {
        setColumnSettings(prev => ({
            ...prev,
            [column]: visible
        }));
    }, [setColumnSettings]);

    // Table columns configuration
    const tableColumns = useMemo(() => {
        const columns = [
            {
                title: (
                    <Checkbox
                        indeterminate={someUsersSelected}
                        checked={allUsersSelected}
                        onChange={(e) => handleSelectAll(e.target.checked)}
                    />
                ),
                key: 'selection',
                width: 50,
                render: (_, user) => (
                    <Checkbox
                        checked={selectedUsers.has(user.id)}
                        onChange={(e) => handleSelectUser(user.id, e.target.checked)}
                    />
                ),
            },
            {
                title: 'Name',
                key: 'name',
                sorter: true,
                ellipsis: true,
                render: (_, user) => (
                    <Space>
                        <span>{user.firstName} {user.lastName}</span>
                        {user.isAdmin && <Tag color="gold">Admin</Tag>}
                    </Space>
                ),
                hidden: !columnSettings.name,
            },
            {
                title: 'Email',
                key: 'email',
                dataIndex: 'email',
                sorter: true,
                ellipsis: true,
                hidden: !columnSettings.email,
            },
            {
                title: 'Status',
                key: 'status',
                render: (_, user) => (
                    <Tag color={user.isActive ? 'green' : 'red'}>
                        {user.isActive ? 'Active' : 'Inactive'}
                    </Tag>
                ),
                hidden: !columnSettings.status,
            },
            {
                title: 'Role',
                key: 'role',
                render: (_, user) => (
                    <Tag color={user.isAdmin ? 'purple' : 'blue'}>
                        {user.isAdmin ? 'Administrator' : 'User'}
                    </Tag>
                ),
                hidden: !columnSettings.role,
            },
            {
                title: 'Last Login',
                key: 'lastLogin',
                sorter: true,
                render: (_, user) => {
                    if (!user.lastLoginAt) return 'Never';
                    const date = new Date(user.lastLoginAt);
                    return (
                        <Tooltip title={date.toLocaleString()}>
                            {date.toLocaleDateString()}
                        </Tooltip>
                    );
                },
                hidden: !columnSettings.lastLogin,
            },
            {
                title: 'Actions',
                key: 'actions',
                width: 120,
                render: (_, user) => (
                    <Space size="small">
                        <Button
                            type="link"
                            size="small"
                            onClick={() => onUserEdit?.(user)}
                        >
                            Edit
                        </Button>
                        <Button
                            type="link"
                            size="small"
                            danger
                            onClick={() => onUserDelete?.(user)}
                        >
                            Delete
                        </Button>
                    </Space>
                ),
                hidden: !columnSettings.actions,
            },
        ];

        return columns.filter(col => !col.hidden);
    }, [
        selectedUsers,
        someUsersSelected,
        allUsersSelected,
        handleSelectAll,
        handleSelectUser,
        columnSettings,
        onUserEdit,
        onUserDelete
    ]);

    // Column settings menu
    const columnSettingsMenu = {
        items: Object.entries(columnSettings).map(([key, visible]) => ({
            key,
            label: (
                <Checkbox
                    checked={visible}
                    onChange={(e) => handleColumnSettingsChange(key, e.target.checked)}
                >
                    {key.charAt(0).toUpperCase() + key.slice(1)}
                </Checkbox>
            ),
        }))
    };

    // Bulk actions menu
    const bulkActionsMenu = {
        items: [
            {
                key: 'delete',
                icon: <DeleteOutlined />,
                label: 'Delete Selected',
                onClick: handleBulkDelete,
                danger: true,
                disabled: selectedUserCount === 0,
            },
            {
                key: 'export',
                icon: <DownloadOutlined />,
                label: 'Export Selected',
                onClick: handleExport,
                disabled: selectedUserCount === 0,
            },
        ]
    };

    // Render toolbar
    const renderToolbar = () => (
        <div className="user-list__toolbar">
            <div className="user-list__toolbar-left">
                <Search
                    ref={searchInputRef}
                    placeholder="Search users..."
                    value={searchQuery}
                    onChange={handleSearchChange}
                    style={{ width: 300 }}
                    allowClear
                />
                
                <Select
                    placeholder="Filter by status"
                    value={filters.status}
                    onChange={(value) => handleFilterChange('status', value)}
                    style={{ width: 150 }}
                    allowClear
                >
                    <Option value="active">Active</Option>
                    <Option value="inactive">Inactive</Option>
                </Select>

                <Select
                    placeholder="Filter by role"
                    value={filters.role}
                    onChange={(value) => handleFilterChange('role', value)}
                    style={{ width: 150 }}
                    allowClear
                >
                    <Option value="admin">Admin</Option>
                    <Option value="user">User</Option>
                </Select>

                <Button
                    icon={<FilterOutlined />}
                    onClick={() => setFilters({})}
                >
                    Clear Filters
                </Button>
            </div>

            <div className="user-list__toolbar-right">
                <Space>
                    {showBulkActions && selectedUserCount > 0 && (
                        <Dropdown menu={bulkActionsMenu}>
                            <Button>
                                Bulk Actions ({selectedUserCount})
                            </Button>
                        </Dropdown>
                    )}

                    <Button
                        type="primary"
                        icon={<UserAddOutlined />}
                        onClick={onUserAdd}
                    >
                        Add User
                    </Button>

                    <Button
                        icon={<ReloadOutlined />}
                        onClick={handleRefresh}
                        loading={loading}
                    >
                        Refresh
                    </Button>

                    <Dropdown menu={columnSettingsMenu}>
                        <Button icon={<SettingOutlined />} />
                    </Dropdown>

                    <Select
                        value={viewMode}
                        onChange={setViewMode}
                        style={{ width: 100 }}
                    >
                        <Option value="table">Table</Option>
                        <Option value="cards">Cards</Option>
                    </Select>
                </Space>
            </div>
        </div>
    );

    // Render content based on view mode
    const renderContent = () => {
        if (loading && users.length === 0) {
            return (
                <div className="user-list__loading">
                    <Spin size="large" />
                </div>
            );
        }

        if (filteredAndSortedUsers.length === 0) {
            return (
                <Empty
                    description={debouncedSearchQuery ? 'No users found matching your search' : 'No users found'}
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
            );
        }

        if (viewMode === 'cards') {
            return (
                <div className="user-list__cards">
                    {filteredAndSortedUsers.map(user => (
                        <UserCard
                            key={user.id}
                            user={user}
                            isSelectable={showBulkActions}
                            isSelected={selectedUsers.has(user.id)}
                            onSelect={(user, selected) => handleSelectUser(user.id, selected)}
                            onEdit={onUserEdit}
                            onDelete={onUserDelete}
                            className="user-list__card"
                        />
                    ))}
                </div>
            );
        }

        return (
            <Table
                columns={tableColumns}
                dataSource={filteredAndSortedUsers}
                rowKey="id"
                pagination={false}
                loading={loading}
                onChange={handleSortChange}
                scroll={{ x: 800 }}
                className="user-list__table"
            />
        );
    };

    // Main render
    return (
        <div className="user-list" ref={listContainerRef}>
            {renderToolbar()}
            
            <div className="user-list__content">
                {renderContent()}
            </div>

            {total > pageSize && (
                <div className="user-list__pagination">
                    <Pagination
                        current={page}
                        total={total}
                        pageSize={pageSize}
                        showSizeChanger
                        showQuickJumper
                        showTotal={(total, range) => 
                            `${range[0]}-${range[1]} of ${total} users`
                        }
                        onChange={onPageChange}
                        onShowSizeChange={onPageSizeChange}
                        pageSizeOptions={['10', '20', '50', '100']}
                    />
                </div>
            )}
        </div>
    );
};

// PropTypes
UserList.propTypes = {
    users: PropTypes.arrayOf(UserShape),
    loading: PropTypes.bool,
    total: PropTypes.number,
    page: PropTypes.number,
    pageSize: PropTypes.number,
    viewMode: PropTypes.oneOf(['table', 'cards']),
    showBulkActions: PropTypes.bool,
    onPageChange: PropTypes.func,
    onPageSizeChange: PropTypes.func,
    onSearch: PropTypes.func,
    onFilter: PropTypes.func,
    onSort: PropTypes.func,
    onUserEdit: PropTypes.func,
    onUserDelete: PropTypes.func,
    onBulkAction: PropTypes.func,
    onUserAdd: PropTypes.func,
    onRefresh: PropTypes.func,
};

export default UserList;

// Custom hook for managing user list state
export const useUserList = (initialUsers = []) => {
    const [users, setUsers] = useState(initialUsers);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [pagination, setPagination] = useState({
        page: 1,
        pageSize: 20,
        total: 0
    });

    const loadUsers = useCallback(async (params = {}) => {
        setLoading(true);
        setError(null);
        try {
            const response = await UserService.getUsers({
                page: pagination.page,
                pageSize: pagination.pageSize,
                ...params
            });
            setUsers(response.users);
            setPagination(prev => ({
                ...prev,
                total: response.total
            }));
        } catch (err) {
            setError(err);
            message.error(`Failed to load users: ${err.message}`);
        } finally {
            setLoading(false);
        }
    }, [pagination.page, pagination.pageSize]);

    return {
        users,
        loading,
        error,
        pagination,
        loadUsers,
        setUsers,
        setPagination,
    };
};