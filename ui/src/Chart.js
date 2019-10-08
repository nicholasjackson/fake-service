import { FlowChartWithState } from "@mrblenny/react-flow-chart";
import React from 'react'
import moment from 'moment'

const data = {
  offset: {
    x: 0,
    y: 0
  },
  nodes:
  {
    node1:
    {
      id: 'node1',
      type: 'output-only',
      position: {
        x: 300,
        y: 100
      },
      properties: {
        name: "nic",
        duration: "2ms"
      },
      ports: {
        port1: {
          id: 'port1',
          type: 'output'
        }
      }
    },
    node2:
    {
      id: 'node2',
      type: 'output-only',
      position: {
        x: 300,
        y: 400
      },
      properties: {
        name: "API",
        duration: "20ms"
      },
      ports: {
        port1: {
          id: 'port1',
          type: 'input'
        }
      }
    }
  },
  links: {
    link1: {
      id: "link1",
      from: {
        nodeId: "node1",
        portId: "port1"
      },
      to: {
        nodeId: "node2",
        portId: "port1"
      }
    }
  },
  selected: {},
  hovered: {}
}

const NodeInnerCustom = ({ node, children, ...otherProps }) => {
  return (
    <div {...otherProps} className="node">
      <b>name:</b> {node.properties.name}<br />
      <b>duration:</b> {node.properties.duration}
    </div >
  )
}

class Timeline extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      url: props.url,
    };
  }

  componentDidMount() {
    this.fetchData();
  }

  fetchData() {
    fetch(this.state.url)
      .then(res => res.json())
      .then(
        (result) => {
          console.log("response from API:", result);
        },
        (error) => {
          console.error("error processing API", error);
        }
      );
  }

  parseElements(result, data) {
    // add the root node

    if (!result.upstream_calls) {
      return data;
    }

    // add sub nodes
    for (var i = 0; i < result.upstream_calls.length; i++) {
      //data.concat(this.parseElements(result.upstream_calls[i], data));
    }

    return data;
  }

  render() {
    return <FlowChartWithState initialValue={data} Components={{ NodeInner: NodeInnerCustom }} />
  }
}

export default Timeline