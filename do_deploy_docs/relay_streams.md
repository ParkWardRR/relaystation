# IMSA Relay Stream Details

The droplet is successfully running at IP `209.38.25.42` in the DigitalOcean Sydney region. It is using an Nginx reverse proxy to relay requests to the geo-blocked CloudFront endpoints.

## Relay URLs

You can simply substitute the CloudFront domains with `209.38.25.42`.

**Primary International Stream (Relayed)**
- `http://209.38.25.42/live/imsa_international/master.m3u8`

**In-Car Camera Streams (Relayed)**
- `http://209.38.25.42/live/imsa_icc_01/master.m3u8`
- `http://209.38.25.42/live/imsa_icc_02/master.m3u8`
- *(replace 01 with 02-14 for other cameras)*

## Testing Status
- ✅ Extracted original m3u8
- ✅ Deployed Droplet with 24h self-destruct 
- ✅ Configured Nginx proxy
- ✅ Tested and verified HTTP 200 OK on relay URL
