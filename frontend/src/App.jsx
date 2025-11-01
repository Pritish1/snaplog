import {useState, useEffect} from 'react';
import './App.css';
import {LogText, HideWindow, Quit, GetSettings, SetSettings, RenderMarkdown, ProcessCommand, ClearAllData} from "../wailsjs/go/main/App";
import {EventsOn} from "../wailsjs/runtime/runtime";

function App() {
    const [text, setText] = useState('');
    const [showSettings, setShowSettings] = useState(false);
    const [previewMode, setPreviewMode] = useState(false);
    const [renderedHtml, setRenderedHtml] = useState('');
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
    const [settings, setSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l',
        theme: 'dark'
    });
    const [tempSettings, setTempSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l',
        theme: 'dark'
    });

    useEffect(() => {
        // Load settings
        GetSettings().then(setSettings);
        
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

        // Cleanup
        return () => {
            document.removeEventListener('keydown', handleGlobalKeyDown);
        };
    }, [showSettings]);

    const handleTextChange = (e) => setText(e.target.value);

    const logText = async () => {
        if (!text.trim()) {
            return;
        }

        // Check for slash commands
        if (text.trim().startsWith('/')) {
            try {
                const command = text.trim();
                await ProcessCommand(command);
                setText(''); // Clear the input
                // Don't hide window for settings command
                if (command !== '/settings') {
                    setTimeout(() => {
                        HideWindow();
                    }, 100);
                }
                return;
            } catch (error) {
                console.error('Error processing command:', error);
                return;
            }
        }

        try {
            await LogText(text);
            setText(''); // Clear the input
            
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
            // Hide window without saving
            HideWindow();
        } else if (e.key === 'Tab' && e.ctrlKey) {
            // Ctrl+Tab to toggle preview mode
            e.preventDefault();
            togglePreviewMode();
        }
    };

    const togglePreviewMode = async () => {
        if (!previewMode && text.trim()) {
            // Render Markdown when switching to preview mode
            try {
                const html = await RenderMarkdown(text);
                setRenderedHtml(html);
            } catch (error) {
                console.error('Error rendering markdown:', error);
                setRenderedHtml('<p>Error rendering markdown</p>');
            }
        }
        setPreviewMode(!previewMode);
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
                case 'alt': return 'Alt';
                case 'shift': return 'Shift';
                default: return m;
            }
        }).join('+');
        return `${modStr}+${key.toUpperCase()}`;
    };

    const handleDeleteAll = async () => {
        try {
            await ClearAllData();
            setShowDeleteConfirm(false);
        } catch (error) {
            console.error('Error deleting all data:', error);
        }
    };

    return (
        <div id="App" className={settings.theme === 'light' ? 'theme-light' : 'theme-dark'}>
            <div className="header">
                <div style={{display: 'flex', gap: '4px', alignItems: 'center'}}>
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
                    <p className="subtitle">Ctrl+Tab: Preview | Esc: Exit</p>
                </div>
            </div>
            
            <div className="input-container">
                <div className="input-header">
                    <span className="mode-indicator">
                        {previewMode ? 'Preview Mode' : 'Edit Mode'}
                    </span>
                    <button 
                        className="preview-toggle"
                        onClick={togglePreviewMode}
                        title="Toggle Preview (Ctrl+Tab)"
                    >
                        {previewMode ? 'Edit' : 'Preview'}
                    </button>
                </div>
                
                {previewMode ? (
                    <div 
                        className="markdown-preview"
                        dangerouslySetInnerHTML={{ 
                            __html: renderedHtml || '<p><em>No content to preview</em></p>' 
                        }}
                    />
                ) : (
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
                    />
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
                                                {modifier.charAt(0).toUpperCase() + modifier.slice(1)}
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

                            {/* Instructions */}
                            <div className="setting-group">
                                <label>Instructions</label>
                                <div className="instructions-box">
                                    <div className="instructions-item">
                                        <strong>Ctrl+Tab:</strong> Toggle preview
                                    </div>
                                    <div className="instructions-item">
                                        <strong>Enter:</strong> Log and hide
                                    </div>
                                    <div className="instructions-item">
                                        <strong>Shift+Enter:</strong> New line
                                    </div>
                                    <div className="instructions-item">
                                        <strong>Esc:</strong> Hide window
                                    </div>
                                    <div className="instructions-item">
                                        <strong>/dash:</strong> Open dashboard
                                    </div>
                                </div>
                            </div>

                            {/* Delete All Data */}
                            <div className="setting-group">
                                <label>Danger Zone</label>
                                {!showDeleteConfirm ? (
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
        </div>
    )
}

export default App
