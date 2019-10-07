import React from 'react';
import './App.css';
import './Chart.js'
import Timeline from './Chart.js';


function App() {
  return (
    <div className="App">
      <header className="App-header">
        <Timeline url="http://localhost:9090" />
      </header>
    </div>
  );
}

export default App;
