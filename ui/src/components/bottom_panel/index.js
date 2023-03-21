import React from 'react';
import './index.css';

import Card from "react-bootstrap/Card";
import CloseButton from 'react-bootstrap/CloseButton';
import PubSub from 'pubsub-js';
import parse from 'html-react-parser';

class BottomPanel extends React.Component {

  constructor(props) {
    super(props)

    this.state = {
      show: false
    }

    this.descriptionClicked = this.descriptionClicked.bind(this)
    this.closeClick = this.closeClick.bind(this)

    PubSub.subscribe("description_clicked", this.descriptionClicked)
  }

  descriptionClicked(msg, e) {
    this.setState({description: e.data, show: true})
  }

  closeClick(e) {
    this.setState({show: false})
  }

  render() {
    const show = this.state.show

    return(
      show &&
      <div className="bottom-panel">
        <Card>
          <Card.Header className='bottom-panel-header'>
            <div className="d-flex">
              <div>Description</div>
              <div className='ms-auto'><CloseButton onClick={this.closeClick}></CloseButton></div>
            </div>
          </Card.Header>
          <Card.Body>
            <Card.Text>
              {parse(this.state.description)}
            </Card.Text>
          </Card.Body>
        </Card>
      </div>
    )
  }
}

export default BottomPanel;