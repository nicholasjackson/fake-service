import React from 'react';
import './App.css';
import './Chart.js'
import Timeline from './Chart.js';

import Navbar from 'react-bootstrap/Navbar'


function App() {
  return (
    <div className="App">
      <Navbar bg="primary" variant="dark" fixed="top">
        <Navbar.Brand><h1>Fake Service</h1></Navbar.Brand>
      </Navbar>
      <Timeline />
    </div>
  );
}

export default App;