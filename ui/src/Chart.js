import { FlowChartWithState } from "@mrblenny/react-flow-chart";
import React from 'react'
import { processData } from './Data'

const NodeInnerCustom = ({ node, children, ...otherProps }) => {
  var className = "node";

  if (node.properties.response !== 200 && node.properties.response !== 0) {
    className = "node-error";
  }

  return (
    <div {...otherProps} className={className}>
      <b>name:</b> {node.properties.name}<br />
      <b>duration:</b> {node.properties.duration}<br />
      <b>type:</b> {node.properties.type}<br />
      <b>response:</b> {node.properties.response}
    </div >
  )
}

class Timeline extends React.Component {

  constructor(props) {
    super(props);

    console.log(process.env.REACT_APP_API_URI);
    var url = (process.env.REACT_APP_API_URI) ? "" + process.env.REACT_APP_API_URI : "http://localhost:9090";
    console.log("API_URI: " + url);

    this.state = {
      url: url,
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