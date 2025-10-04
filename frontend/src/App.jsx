import {useState, useEffect} from 'react';
import './App.css';
import {LogText, HideWindow, Quit, GetSettings, SetSettings, RenderMarkdown} from "../wailsjs/go/main/App";
import {EventsOn} from "../wailsjs/runtime/runtime";

function App() {
    const [text, setText] = useState('');
    const [showSettings, setShowSettings] = useState(false);
    const [showInstructions, setShowInstructions] = useState(false);
    const [previewMode, setPreviewMode] = useState(false);
    const [renderedHtml, setRenderedHtml] = useState('');
    const [settings, setSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l'
    });
    const [tempSettings, setTempSettings] = useState({
        hotkey_modifiers: ['ctrl', 'shift'],
        hotkey_key: 'l'
    });

    useEffect(() => {
        // Load settings
        GetSettings().then(setSettings);
        
        // Listen for open-settings event from system menu
        EventsOn("open-settings", () => {
            setTempSettings({...settings});
            setShowSettings(true);
        });
        
        // Listen for show-instructions event from system menu
        EventsOn("show-instructions", () => {
            setShowInstructions(true);
        });
    }, []);

    const handleTextChange = (e) => setText(e.target.value);

    const logText = async () => {
        if (!text.trim()) {
            return;
        }

        // Check for special commands
        if (text.trim() === '/dash') {
            try {
                // Call the dashboard generation directly
                await LogText('/dash');
                setText(''); // Clear the input
                // Hide the window immediately
                setTimeout(() => {
                    HideWindow();
                }, 100);
                return;
            } catch (error) {
                console.error('Error generating dashboard:', error);
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

    const closeInstructions = () => {
        setShowInstructions(false);
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

    return (
        <div id="App">
            <div className="header">
                <h1>SnapLog</h1>
                <p className="subtitle">Quick text logging utility</p>
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
                            <div className="setting-group">
                                <label>Hotkey Configuration</label>
                                <div className="hotkey-config">
                                    <div className="modifiers">
                                        <label>Modifiers:</label>
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
                                    
                                    <div className="key-selection">
                                        <label>Key:</label>
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
                                    
                                    <div className="hotkey-preview">
                                        <strong>Preview:</strong> {formatHotkey(tempSettings.hotkey_modifiers, tempSettings.hotkey_key)}
                                    </div>
                                </div>
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
                <div className="modal-overlay" onClick={closeInstructions}>
                    <div className="modal-content" onClick={(e) => e.stopPropagation()}>
                        <div className="modal-header">
                            <h2>Instructions</h2>
                            <button className="close-btn" onClick={closeInstructions}>×</button>
                        </div>
                        
                        <div className="modal-body">
                            <div className="instructions-content">
                                <h3>Keyboard Shortcuts</h3>
                                <div className="shortcut-list">
                                    <div className="shortcut-item">
                                        <strong>Enter:</strong> Log text and hide window
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Shift+Enter:</strong> Create a new line
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Ctrl+Tab:</strong> Toggle Markdown preview
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Esc:</strong> Hide window without saving
                                    </div>
                                </div>
                                
                                <h3>Markdown Support</h3>
                                <div className="shortcut-list">
                                    <div className="shortcut-item">
                                        <strong>Headers:</strong> # H1, ## H2, ### H3
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Lists:</strong> - Bullet, 1. Numbered
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Links:</strong> [text](url)
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Bold:</strong> **bold text**
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Italic:</strong> *italic text*
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Code:</strong> `inline code`
                                    </div>
                                </div>
                                
                                <h3>System Tray</h3>
                                <div className="shortcut-list">
                                    <div className="shortcut-item">
                                        <strong>Right-click tray icon:</strong> Access menu
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Show Window:</strong> Display main window
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Settings:</strong> Configure hotkey
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Instructions:</strong> Show this help
                                    </div>
                                    <div className="shortcut-item">
                                        <strong>Quit:</strong> Exit application
                                    </div>
                                </div>
                                
                                <h3>Current Hotkey</h3>
                                <div className="current-hotkey">
                                    <strong>{formatHotkey(settings.hotkey_modifiers, settings.hotkey_key)}</strong> - Show window
                                </div>
                            </div>
                        </div>
                        
                        <div className="modal-footer">
                            <button className="save-btn" onClick={closeInstructions}>Got it!</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

export default App
