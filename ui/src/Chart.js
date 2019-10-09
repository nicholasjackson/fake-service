import { FlowChartWithState } from "@mrblenny/react-flow-chart";
import React from 'react'
import { processData } from './Data'

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
      loaded: false,
    };
  }

  componentWillMount() {
    this.fetchData();
  }

  fetchData() {
    fetch(this.state.url)
      .then(res => res.json())
      .then(
        (result) => {
          console.log("response from API:", result);
          var data = processData(result);

          console.log("data to map", data);

          this.setState({ "data": data, loaded: true });
        },
        (error) => {
          console.error("error processing API", error);
        }
      );
  }

  render() {
    if (this.state.loaded === true) {
      return <FlowChartWithState initialValue={this.state.data} Components={{ NodeInner: NodeInnerCustom }} />
    }

    return null
  }
}

export default Timeline