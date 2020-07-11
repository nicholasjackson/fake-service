import React from 'react';
import './App.css';
import './Chart.js'
import Timeline from './Chart.js';

import Navbar from 'react-bootstrap/Navbar';
import FormControl from 'react-bootstrap/FormControl';
import Form from 'react-bootstrap/Form';
import Button from 'react-bootstrap/Button';

class App extends React.Component {

  constructor(props) {
    super(props);

    console.log(process.env.REACT_APP_API_URI);
    var baseUrl = (process.env.REACT_APP_API_URI) ? "" + process.env.REACT_APP_API_URI : "http://localhost:9090";
    console.log("API_URI: " + baseUrl);

    this.state = {
      baseUrl: baseUrl,
      url: baseUrl,
      refresh: new Date().getMilliseconds()
    };

    this.pathChanged = this.pathChanged.bind(this);
    this.goClick = this.goClick.bind(this);
  }

  pathChanged(e) {
    this.setState({ tempPath: e.target.value });
  }

  goClick(e) {
    console.log(this.state.tempPath);
    // calculate the path
    var newURL = this.state.baseUrl;

    // ensure the new url always ends in a / before adding the path
    if (!newURL.endsWith("/")) {
      newURL = newURL + "/";
    }

    // ensure the new path does not start with a /
    var path = (this.state.tempPath === undefined) ? "/" : this.state.tempPath;
    if (path.startsWith("/")) {
      newURL = newURL + path.slice(1);
    } else {
      newURL = newURL + path;
    }

    this.setState({ url: newURL, refresh: new Date().getMilliseconds() });
  }

  render() {
    return (
      <div className="App">
        <Navbar bg="dark" variant="dark" fixed="top">
          <Navbar.Brand><h1>Fake Service</h1></Navbar.Brand>
          <Form inline>
            <FormControl style={{width:"600px"}} type="text" placeholder="/" className="mr-sm-4" onChange={this.pathChanged}/>
            <Button variant="outline-light" onClick={this.goClick}>Go</Button>
          </Form>
        </Navbar>
        <Timeline url={this.state.url} refresh={this.state.refresh} />
      </div>
    );
  }
}

export default App;