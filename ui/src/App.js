import React from 'react';
import './App.css';
import './Chart.js'
import Timeline from './Chart.js';

import Navbar from 'react-bootstrap/Navbar';
import Form from 'react-bootstrap/Form';
import Button from 'react-bootstrap/Button';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';

import BottomPanel from './components/bottom_panel';

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
        <Navbar bg="light" fixed="top">
          <Container className='align-middle'>
            <Navbar.Brand>
              <img src="server.png" className="d-inline-block align-top App-logo" alt="server"/>
              <div className='d-inline-block App-header-text'>Fake Service</div>
            </Navbar.Brand>
            <Navbar.Toggle />
            <Navbar.Collapse>
              <Form className="form-custom align-middle">
                <Form.Group as={Row} className="mb-3 align-middle d-lg-flex">
                    <Form.Label column sm="1">path:</Form.Label>
                    <Col sm="9">
                      <Form.Control column type="text" placeholder="/" onChange={this.pathChanged}/>
                    </Col>
                    <Col sm="1">
                      <Button column variant="outline-success" onClick={this.goClick}>Go</Button>
                    </Col>
                </Form.Group>
              </Form>
            </Navbar.Collapse>
          </Container>
        </Navbar>
        <Timeline url={this.state.url} refresh={this.state.refresh} />
        <BottomPanel></BottomPanel>
      </div>
    );
  }
}

export default App;