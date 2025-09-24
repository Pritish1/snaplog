import {useState, useEffect} from 'react';
import './App.css';
import {LogText, GetLogFilePath, HideWindow, Quit} from "../wailsjs/go/main/App";

function App() {
    const [text, setText] = useState('');
    const [status, setStatus] = useState('Ready to log text...');
    const [logPath, setLogPath] = useState('');

    useEffect(() => {
        // Get the log file path when component mounts
        GetLogFilePath().then(setLogPath);
    }, []);

    const handleTextChange = (e) => setText(e.target.value);

    const logText = async () => {
        if (!text.trim()) {
            setStatus('Please enter some text to log');
            return;
        }

        try {
            await LogText(text);
            setStatus(`✅ Logged: "${text}"`);
            setText(''); // Clear the input
            
            // Hide the window after successful logging
            setTimeout(() => {
                HideWindow();
            }, 500); // Small delay to show the success message
            
        } catch (error) {
            setStatus(`❌ Error: ${error.message}`);
        }
    };

    const handleKeyPress = (e) => {
        if (e.key === 'Enter') {
            logText();
        }
    };

    const quitApp = () => {
        Quit();
    };

    return (
        <div id="App">
            <div className="header">
                <h1>SnapLog</h1>
                <p className="subtitle">Quick text logging utility</p>
            </div>
            
            <div className="status">{status}</div>
            
            <div className="input-container">
                <textarea
                    id="textInput"
                    className="text-input"
                    value={text}
                    onChange={handleTextChange}
                    onKeyPress={handleKeyPress}
                    placeholder="Enter text to log..."
                    rows="4"
                    autoFocus
                />
                <div className="button-group">
                    <button className="log-btn" onClick={logText}>
                        Log Text
                    </button>
                    <button className="quit-btn" onClick={quitApp}>
                        Quit App
                    </button>
                </div>
            </div>
            
            {logPath && (
                <div className="log-info">
                    <small>Log file: {logPath}</small>
                </div>
            )}
            
            <div className="instructions">
                <p><strong>Hotkey:</strong> Ctrl+Shift+L to show this window</p>
                <p><strong>Tip:</strong> Press Enter to log text and hide window</p>
                <p><strong>Background:</strong> App runs in background, always ready</p>
                <p><strong>Quit:</strong> Click "Quit App" button to exit completely</p>
            </div>
        </div>
    )
}

export default App
