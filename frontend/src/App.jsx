import {useState, useEffect} from 'react';
import './App.css';
import {LogText, HideWindow, Quit, GetSettings, SetSettings, RenderMarkdown, ProcessCommand, ClearAllData, GetDatabasePath, UpdateEntry, DeleteEntry} from "../wailsjs/go/main/App";
import {EventsOn} from "../wailsjs/runtime/runtime";

function App() {
    const [text, setText] = useState('');
    const [charCount, setCharCount] = useState(0);
    const [showSettings, setShowSettings] = useState(false);
    const [showInstructions, setShowInstructions] = useState(false);
    const [previewMode, setPreviewMode] = useState(false);
    const [renderedHtml, setRenderedHtml] = useState('');
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [deleteSuccess, setDeleteSuccess] = useState(false);
    const [databasePath, setDatabasePath] = useState('');
    const [editingEntryId, setEditingEntryId] = useState(null);
    const [deleteConfirmId, setDeleteConfirmId] = useState(null);
    const [deleteConfirmPreview, setDeleteConfirmPreview] = useState('');
    
    // Detect macOS
    const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0 || navigator.userAgent.toUpperCase().indexOf('MAC') >= 0;
    const [settings, setSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l',
        theme: 'dark',
        dashboard_port: 37564
    });
    const [tempSettings, setTempSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l',
        theme: 'dark',
        dashboard_port: 37564
    });

    useEffect(() => {
        // Load settings
        GetSettings().then(setSettings);
        
        // Load file paths
        GetDatabasePath().then(setDatabasePath);
        
        // Listen for open-settings event
        EventsOn("open-settings", () => {
            // Reload settings to get current values
            GetSettings().then(currentSettings => {
                setSettings(currentSettings);
                setTempSettings({...currentSettings});
                setShowSettings(true);
            });
        });

        // Listen for first-run setup
        EventsOn("show-first-run-setup", () => {
            // Reload settings to get current values
            GetSettings().then(currentSettings => {
                setSettings(currentSettings);
                setTempSettings({...currentSettings});
                setShowSettings(true);
            });
        });

        // Global key listener for Esc key
        const handleGlobalKeyDown = (e) => {
            if (e.key === 'Escape') {
                // Close settings modal if open
                if (showSettings) {
                    setShowSettings(false);
                    return;
                }
                // Otherwise hide window
                HideWindow();
            }
        };

        // Add global event listener
        document.addEventListener('keydown', handleGlobalKeyDown);

        // Focus textarea when window gains focus (e.g., when hotkey is pressed)
        const handleWindowFocus = () => {
            if (!showSettings && !deleteConfirmId && !editingEntryId) {
                // Small delay to ensure the textarea is rendered
                setTimeout(() => {
                    const textInput = document.getElementById('textInput');
                    if (textInput) {
                        textInput.focus();
                    }
                }, 100);
            }
        };

        window.addEventListener('focus', handleWindowFocus);

        // Cleanup
        return () => {
            document.removeEventListener('keydown', handleGlobalKeyDown);
            window.removeEventListener('focus', handleWindowFocus);
        };
    }, [showSettings, deleteConfirmId, editingEntryId]);

    // Focus textarea when component mounts or window becomes visible
    useEffect(() => {
        // Initial focus on mount
        const initialTimer = setTimeout(() => {
            const textInput = document.getElementById('textInput');
            if (textInput && !showSettings && !deleteConfirmId && !editingEntryId) {
                textInput.focus();
            }
        }, 200);
        
        return () => clearTimeout(initialTimer);
    }, []); // Run only on mount

    // Focus textarea when window becomes visible (not in settings or delete confirmation)
    useEffect(() => {
        if (!showSettings && !deleteConfirmId && !editingEntryId) {
            // Small delay to ensure the textarea is rendered
            const timer = setTimeout(() => {
                const textInput = document.getElementById('textInput');
                if (textInput) {
                    textInput.focus();
                }
            }, 150);
            return () => clearTimeout(timer);
        }
    }, [showSettings, deleteConfirmId, editingEntryId]);

    // Also focus when edit mode ends
    useEffect(() => {
        if (!editingEntryId && !showSettings && !deleteConfirmId) {
            const timer = setTimeout(() => {
                const textInput = document.getElementById('textInput');
                if (textInput) {
                    textInput.focus();
                }
            }, 100);
            return () => clearTimeout(timer);
        }
    }, [editingEntryId, showSettings, deleteConfirmId]);

    const MAX_TEXT_LENGTH = 50000;
    
    const handleTextChange = (e) => {
        const newText = e.target.value;
        if (newText.length <= MAX_TEXT_LENGTH) {
            setText(newText);
            setCharCount(newText.length);
        }
    };

    const logText = async () => {
        // If text is empty, just minimize the window
        if (!text.trim()) {
            // If in edit mode, cancel edit
            if (editingEntryId) {
                setEditingEntryId(null);
            }
            setTimeout(() => {
                HideWindow();
            }, 100);
            return;
        }

        // Check for recognized slash commands
        const trimmedText = text.trim();
        const recognizedCommands = ['/dash', '/settings', '/editprev', '/delprev'];
        
        if (trimmedText.startsWith('/') && recognizedCommands.includes(trimmedText)) {
            try {
                await ProcessCommand(trimmedText);
                // If ProcessCommand succeeds, it shouldn't happen for editprev/delprev
                // They should return errors with special format
                setText(''); // Clear the input
                setCharCount(0); // Reset character count
                // Don't hide window for settings command
                if (trimmedText !== '/settings') {
                    setTimeout(() => {
                        HideWindow();
                    }, 100);
                }
                return;
            } catch (error) {
                // ProcessCommand returns an error, but we use it to pass data for editprev/delprev
                const errorMsg = error?.message || error?.toString() || '';
                
                if (errorMsg.startsWith('EDIT_MODE:')) {
                    // Parse EDIT_MODE:<id>:<content>
                    const parts = errorMsg.split(':');
                    if (parts.length >= 3) {
                        const entryId = parseInt(parts[1]);
                        const content = parts.slice(2).join(':'); // Rejoin in case content has colons
                        setEditingEntryId(entryId);
                        setText(content);
                        // Don't hide window, allow editing
                        return;
                    }
                } else if (errorMsg.startsWith('DELETE_CONFIRM:')) {
                    // Parse DELETE_CONFIRM:<id>:<preview>
                    const parts = errorMsg.split(':');
                    if (parts.length >= 3) {
                        const entryId = parseInt(parts[1]);
                        const preview = parts.slice(2).join(':'); // Rejoin in case preview has colons
                        setDeleteConfirmId(entryId);
                        setDeleteConfirmPreview(preview);
                        setText(''); // Clear the input
                        setCharCount(0); // Reset character count
                        // Don't hide window, show confirmation
                        return;
                    }
                } else {
                    // Actual error or other command
                    console.error('Error processing command:', errorMsg);
                    setText('');
                    setCharCount(0); // Reset character count
                    if (trimmedText !== '/settings') {
                        setTimeout(() => {
                            HideWindow();
                        }, 100);
                    }
                }
                return;
            }
        }

        // Check for edit/delete commands
        if (trimmedText.startsWith('/edit ') || trimmedText.startsWith('/delete ')) {
            try {
                await ProcessCommand(trimmedText);
                // If ProcessCommand succeeds, it shouldn't happen for edit/delete
                // They should return errors with special format
                setText('');
                setCharCount(0); // Reset character count
                setTimeout(() => {
                    HideWindow();
                }, 100);
            } catch (error) {
                // ProcessCommand returns an error, but we use it to pass data
                const errorMsg = error?.message || error?.toString() || '';
                
                if (errorMsg.startsWith('EDIT_MODE:')) {
                    // Parse EDIT_MODE:<id>:<content>
                    const parts = errorMsg.split(':');
                    if (parts.length >= 3) {
                        const entryId = parseInt(parts[1]);
                        const content = parts.slice(2).join(':'); // Rejoin in case content has colons
                        setEditingEntryId(entryId);
                        setText(content);
                        // Don't hide window, allow editing
                        return;
                    }
                } else if (errorMsg.startsWith('DELETE_CONFIRM:')) {
                    // Parse DELETE_CONFIRM:<id>:<preview>
                    const parts = errorMsg.split(':');
                    if (parts.length >= 3) {
                        const entryId = parseInt(parts[1]);
                        const preview = parts.slice(2).join(':'); // Rejoin in case preview has colons
                        setDeleteConfirmId(entryId);
                        setDeleteConfirmPreview(preview);
                        setText(''); // Clear the input
                        setCharCount(0); // Reset character count
                        // Don't hide window, show confirmation
                        return;
                    }
                } else {
                    // Actual error
                    console.error('Error processing command:', errorMsg);
                    setText('');
                    setCharCount(0); // Reset character count
                    setTimeout(() => {
                        HideWindow();
                    }, 100);
                }
                return;
            }
        }

        // If in edit mode, update the entry
        if (editingEntryId) {
            try {
                await UpdateEntry(editingEntryId, text);
                setText(''); // Clear the input
                setCharCount(0); // Reset character count
                setEditingEntryId(null); // Exit edit mode
                // Don't hide window - let user continue working
            } catch (error) {
                console.error('Error updating entry:', error);
                // Show error but stay in edit mode
            }
            return;
        }

        // Log as regular text (even if it starts with / but isn't a recognized command)
        try {
            await LogText(text);
            setText(''); // Clear the input
            setCharCount(0); // Reset character count
            
            // Hide the window after successful logging
            setTimeout(() => {
                HideWindow();
            }, 100); // Small delay to ensure text is logged
            
        } catch (error) {
            console.error('Error logging text:', error);
        }
    };

    const handleKeyPress = (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault(); // Prevent default behavior (new line)
            logText();
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            // If in edit mode, cancel edit
            if (editingEntryId) {
                setEditingEntryId(null);
                setText('');
                setCharCount(0);
            }
            // If delete confirmation is open, cancel it
            if (deleteConfirmId) {
                handleDeleteCancel();
                return;
            }
            // Hide window without saving
            HideWindow();
        } else if (e.key === 'Tab' && (e.ctrlKey || e.metaKey)) {
            // Ctrl+Tab (Windows/Linux) or Cmd+Tab (macOS) to toggle preview mode
            e.preventDefault();
            togglePreviewMode();
        }
    };

    const togglePreviewMode = async () => {
        const newPreviewMode = !previewMode;
        if (newPreviewMode && text.trim()) {
            // Render Markdown when switching to preview mode
            try {
                const html = await RenderMarkdown(text);
                setRenderedHtml(html);
            } catch (error) {
                console.error('Error rendering markdown:', error);
                setRenderedHtml('<p>Error rendering markdown</p>');
            }
        }
        setPreviewMode(newPreviewMode);
    };

    const saveSettings = async () => {
        try {
            await SetSettings(tempSettings);
            setSettings({...tempSettings});
            setShowSettings(false);
        } catch (error) {
            console.error('Error saving settings:', error);
        }
    };

    const closeSettings = () => {
        setShowSettings(false);
    };

    const toggleModifier = (modifier) => {
        const newModifiers = tempSettings.hotkey_modifiers.includes(modifier)
            ? tempSettings.hotkey_modifiers.filter(m => m !== modifier)
            : [...tempSettings.hotkey_modifiers, modifier];
        setTempSettings({...tempSettings, hotkey_modifiers: newModifiers});
    };

    const formatHotkey = (modifiers, key) => {
        const modStr = modifiers.map(m => {
            switch(m) {
                case 'ctrl': return 'Ctrl';
                case 'cmd': return 'Cmd';
                case 'alt': return isMac ? 'Option' : 'Alt';
                case 'shift': return 'Shift';
                default: return m;
            }
        }).join('+');
        return `${modStr}+${key.toUpperCase()}`;
    };
    
    const getModifierLabel = (modifier) => {
        switch(modifier) {
            case 'ctrl': return 'Ctrl';
            case 'cmd': return 'Cmd';
            case 'alt': return isMac ? 'Option' : 'Alt';
            case 'shift': return 'Shift';
            default: return modifier.charAt(0).toUpperCase() + modifier.slice(1);
        }
    };

    const handleDeleteAll = async () => {
        try {
            await ClearAllData();
            setShowDeleteConfirm(false);
            setDeleteSuccess(true);
            // Reset success message after 3 seconds
            setTimeout(() => {
                setDeleteSuccess(false);
            }, 3000);
        } catch (error) {
            console.error('Error deleting all data:', error);
            setShowDeleteConfirm(false);
        }
    };


    const handleDeleteConfirm = async () => {
        if (!deleteConfirmId) return;
        
        try {
            await DeleteEntry(deleteConfirmId);
            setDeleteConfirmId(null);
            setDeleteConfirmPreview('');
            setText('');
            setCharCount(0); // Reset character count
            // Keep window open after successful delete - don't minimize
        } catch (error) {
            console.error('Error deleting entry:', error);
            // Show error but keep confirmation open
        }
    };

    const handleDeleteCancel = () => {
        setDeleteConfirmId(null);
        setDeleteConfirmPreview('');
        setText('');
        setCharCount(0); // Reset character count
        // Don't hide window on cancel - let user continue working
    };

    return (
        <div id="App" className={settings.theme === 'light' ? 'theme-light' : 'theme-dark'}>
            <div className="header">
                <div style={{display: 'flex', gap: '4px', alignItems: 'center'}}>
                    <button 
                        className="info-btn"
                        onClick={() => setShowInstructions(true)}
                        title="Instructions"
                    >
                        ℹ️
                    </button>
                    <button 
                        className="settings-btn"
                        onClick={async () => {
                            const currentSettings = await GetSettings();
                            setSettings(currentSettings);
                            setTempSettings({...currentSettings});
                            setShowSettings(true);
                        }}
                        title="Settings"
                    >
                        ⚙️
                    </button>
                    <p className="subtitle">{isMac ? 'Cmd+Tab' : 'Ctrl+Tab'}: Preview | Esc: Exit</p>
                </div>
            </div>
            
            {deleteConfirmId && (
                <div className="delete-confirm-overlay">
                    <div className="delete-confirm-dialog">
                        <h3>Delete Entry?</h3>
                        <p className="delete-preview">Preview: {deleteConfirmPreview}</p>
                        <div className="delete-confirm-buttons">
                            <button className="delete-confirm-btn" onClick={handleDeleteConfirm}>
                                Delete
                            </button>
                            <button className="delete-cancel-btn" onClick={handleDeleteCancel}>
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}
            
            {editingEntryId && (
                <div className="edit-mode-banner">
                    Editing entry #{editingEntryId} - Press Enter to save, Esc to cancel
                </div>
            )}
            
            <div className="input-container">
                <div className="input-header">
                    <span className="mode-indicator">
                        {editingEntryId ? `Editing Entry #${editingEntryId}` : (previewMode ? 'Preview Mode' : 'Edit Mode')}
                    </span>
                    <button 
                        className="preview-toggle"
                        onClick={togglePreviewMode}
                        title={`Toggle Preview (${isMac ? 'Cmd+Tab' : 'Ctrl+Tab'})`}
                    >
                        {previewMode ? 'Edit' : 'Preview'}
                    </button>
                </div>
                
                {previewMode ? (
                    <div 
                        className="markdown-preview"
                        onKeyDown={handleKeyDown}
                        tabIndex={0}
                        dangerouslySetInnerHTML={{ 
                            __html: renderedHtml || '<p><em>No content to preview</em></p>' 
                        }}
                    />
                ) : (
                    <div className="textarea-wrapper">
                        <textarea
                            id="textInput"
                            className="text-input"
                            value={text}
                            onChange={handleTextChange}
                            onKeyPress={handleKeyPress}
                            onKeyDown={handleKeyDown}
                            placeholder="Enter text to log... (Markdown supported)"
                            rows="4"
                            autoFocus
                            maxLength={MAX_TEXT_LENGTH}
                        />
                        <div className="char-counter">
                            {charCount.toLocaleString()}/{MAX_TEXT_LENGTH.toLocaleString()}
                        </div>
                    </div>
                )}
            </div>

            {/* Settings Modal */}
            {showSettings && (
                <div className="modal-overlay" onClick={closeSettings}>
                    <div className="modal-content" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2>Settings</h2>
                            <button className="close-btn" onClick={closeSettings}>×</button>
                        </div>
                        
                        <div className="modal-body">
                            {/* Hotkey Configuration - Compact */}
                            <div className="setting-group">
                                <label>Hotkey</label>
                                <div className="hotkey-config-compact">
                                    <div className="modifiers-compact">
                                        {['ctrl', 'alt', 'shift'].map(modifier => (
                                            <label key={modifier} className="checkbox-label">
                                                <input
                                                    type="checkbox"
                                                    checked={tempSettings.hotkey_modifiers.includes(modifier)}
                                                    onChange={() => toggleModifier(modifier)}
                                                />
                                                {getModifierLabel(modifier)}
                                            </label>
                                        ))}
                                    </div>
                                    <div className="key-selection-compact">
                                        <select 
                                            value={tempSettings.hotkey_key}
                                            onChange={(e) => setTempSettings({...tempSettings, hotkey_key: e.target.value})}
                                        >
                                            <option value="l">L</option>
                                            <option value="s">S</option>
                                            <option value="t">T</option>
                                            <option value="n">N</option>
                                            <option value="space">Space</option>
                                        </select>
                                    </div>
                                    <div className="hotkey-preview-compact">
                                        {formatHotkey(tempSettings.hotkey_modifiers, tempSettings.hotkey_key)}
                                    </div>
                                </div>
                            </div>

                            {/* Theme Selection */}
                            <div className="setting-group">
                                <label>Theme</label>
                                <div className="theme-toggle">
                                    <label className="radio-label">
                                        <input
                                            type="radio"
                                            name="theme"
                                            checked={tempSettings.theme === 'dark'}
                                            onChange={() => setTempSettings({...tempSettings, theme: 'dark'})}
                                        />
                                        Dark
                                    </label>
                                    <label className="radio-label">
                                        <input
                                            type="radio"
                                            name="theme"
                                            checked={tempSettings.theme === 'light'}
                                            onChange={() => setTempSettings({...tempSettings, theme: 'light'})}
                                        />
                                        Light
                                    </label>
                                </div>
                            </div>

                            {/* Dashboard Port Configuration */}
                            <div className="setting-group">
                                <label>Dashboard Port</label>
                                <p className="setting-note">Port for the dashboard HTTP server. If the port is in use, SnapLog will automatically try nearby ports.</p>
                                <input
                                    type="number"
                                    min="1024"
                                    max="65535"
                                    value={tempSettings.dashboard_port || 37564}
                                    onKeyDown={(e) => {
                                        const value = e.target.value;
                                        const isControlKey = ['Backspace', 'Delete', 'ArrowLeft', 'ArrowRight', 'Tab', 'Home', 'End'].includes(e.key);
                                        const isModifierKey = e.ctrlKey || e.metaKey || e.altKey;
                                        if (value.length >= 5 && !isControlKey && !isModifierKey && /^\d$/.test(e.key)) {
                                            e.preventDefault();
                                        }
                                    }}
                                    onChange={(e) => {
                                        let value = e.target.value;
                                        if (value.length > 5) {
                                            value = value.slice(0, 5);
                                        }
                                        if (value === '' || value === '-') {
                                            return;
                                        }
                                        const port = parseInt(value);
                                        if (!isNaN(port)) {
                                            setTempSettings({...tempSettings, dashboard_port: port});
                                        }
                                    }}
                                    style={{
                                        width: '100%',
                                        padding: '8px',
                                        fontSize: '14px',
                                        border: '1px solid #ddd',
                                        borderRadius: '4px'
                                    }}
                                />
                            </div>

                            {/* Delete All Data */}
                            <div className="setting-group">
                                <label>Danger Zone</label>
                                {deleteSuccess ? (
                                    <div className="delete-success">
                                        <p style={{color: '#27ae60', margin: 0}}>✓ All data deleted successfully</p>
                                    </div>
                                ) : !showDeleteConfirm ? (
                                    <button className="danger-btn" onClick={() => setShowDeleteConfirm(true)}>
                                        Delete All Logged Data
                                    </button>
                                ) : (
                                    <div className="delete-confirm">
                                        <p>Are you sure? This cannot be undone.</p>
                                        <div className="delete-actions">
                                            <button className="danger-btn-confirm" onClick={handleDeleteAll}>
                                                Yes, Delete All
                                            </button>
                                            <button className="cancel-delete" onClick={() => setShowDeleteConfirm(false)}>
                                                Cancel
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </div>
                        </div>
                        
                        <div className="modal-footer">
                            <button className="cancel-btn" onClick={closeSettings}>Cancel</button>
                            <button className="save-btn" onClick={saveSettings}>Save Settings</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Instructions Modal */}
            {showInstructions && (
                <div className="modal-overlay" onClick={() => setShowInstructions(false)}>
                    <div className="modal-content instructions-modal" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2>Instructions</h2>
                            <button className="close-btn" onClick={() => setShowInstructions(false)}>×</button>
                        </div>
                        
                        <div className="modal-body instructions-body">
                            <div className="instructions-section">
                                <h3>Keyboard Shortcuts</h3>
                                <div className="instructions-list">
                                    <div className="instruction-item">
                                        <strong>Enter:</strong> Log text and hide window
                                    </div>
                                    <div className="instruction-item">
                                        <strong>Shift+Enter:</strong> Insert new line
                                    </div>
                                    <div className="instruction-item">
                                        <strong>Esc:</strong> Hide window without logging
                                    </div>
                                </div>
                            </div>

                            <div className="instructions-section">
                                <h3>Commands</h3>
                                <div className="instructions-list">
                                    <div className="instruction-item">
                                        <code>/dash</code> - Open dashboard with all logs
                                    </div>
                                    <div className="instruction-item">
                                        <code>/settings</code> - Open settings window
                                    </div>
                                    <div className="instruction-item">
                                        <code>/edit &lt;id&gt;</code> - Edit an entry by ID
                                    </div>
                                    <div className="instruction-item">
                                        <code>/editprev</code> - Edit the previous (most recent) entry
                                    </div>
                                    <div className="instruction-item">
                                        <code>/delete &lt;id&gt;</code> - Delete an entry by ID
                                    </div>
                                    <div className="instruction-item">
                                        <code>/delprev</code> - Delete the previous (most recent) entry
                                    </div>
                                </div>
                            </div>

                            <div className="instructions-section">
                                <h3>File Locations</h3>
                                <div className="instructions-list">
                                    <div className="instruction-item">
                                        <strong>Database:</strong> <code className="path">{databasePath}</code>
                                    </div>
                                </div>
                            </div>

                            <div className="instructions-section">
                                <h3>Tips</h3>
                                <div className="instructions-list">
                                    <div className="instruction-item">
                                        • Type your hotkey to quickly log thoughts
                                    </div>
                                    <div className="instruction-item">
                                        • Use Markdown formatting for rich text logs
                                    </div>
                                    <div className="instruction-item">
                                        • Preview before logging to check formatting
                                    </div>
                                    <div className="instruction-item">
                                        • Use <code>/dash</code> to view all your logs in a web dashboard
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <div className="modal-footer">
                            <button className="save-btn" onClick={() => setShowInstructions(false)}>Got it!</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

export default App
